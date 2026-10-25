// deploy-dashboard: single-page Kubernetes deployment manager.
//
// Uses the standard library only (no client-go) so it can be built in
// restricted environments. Talks to the Kubernetes API server directly
// over HTTPS using the in-cluster service account token.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"deploy-dashboard"
)

const (
	tokenFile   = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	certFile    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	defaultNS   = "default"
	defaultPort = ":8080"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "kubeconfig path (empty = in-cluster)")
	namespace  = flag.String("namespace", defaultNS, "namespace to watch")
	listenAddr = flag.String("listen", defaultPort, "listen address")
	basePath   = flag.String("base-path", "", "serve the SPA + API under this base path (e.g. /app). Empty = serve at /")
)

// ---------- k8s API client (stdlib only) ----------

type k8sClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newK8sClient() (*k8sClient, error) {
	var baseURL, token, caFile string

	if *kubeconfig != "" {
		// Minimal kubeconfig parse: just grab server + token from a file.
		// For in-cluster we use the standard files.
		b, err := os.ReadFile(*kubeconfig)
		if err != nil {
			return nil, err
		}
		// Very naive: look for "server:" and "token:" lines.
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "server:") {
				baseURL = strings.TrimPrefix(line, "server:")
				baseURL = strings.TrimSpace(baseURL)
			}
			if strings.HasPrefix(line, "token:") {
				token = strings.TrimSpace(strings.TrimPrefix(line, "token:"))
			}
		}
		if baseURL == "" {
			return nil, fmt.Errorf("no server in kubeconfig %s", *kubeconfig)
		}
	} else {
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port := os.Getenv("KUBERNETES_SERVICE_PORT")
		if host == "" || port == "" {
			return nil, fmt.Errorf("not running in-cluster and no --kubeconfig given")
		}
		baseURL = "https://" + host + ":" + port
		var err error
		tokenBytes, err := os.ReadFile(tokenFile)
		if err != nil {
			return nil, fmt.Errorf("read token: %w", err)
		}
		token = strings.TrimSpace(string(tokenBytes))
		caFile = certFile
	}

	// Build HTTP client with CA cert.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if caFile != "" {
		ca, err := os.ReadFile(caFile)
		if err == nil {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(ca)
			transport.TLSClientConfig = &tls.Config{RootCAs: pool}
		}
	}
	// For local testing without a CA, allow insecure.
	if os.Getenv("KUBERNETES_INSECURE") == "1" {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &k8sClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Transport: transport, Timeout: 30 * time.Second},
	}, nil
}

func (c *k8sClient) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		if r, ok := body.(io.Reader); ok {
			reqBody = r
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	// K8s API requires specific content types for PATCH requests
	if method == "PATCH" {
		req.Header.Set("Content-Type", "application/merge-patch+json")
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, string(b))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// ---------- k8s types (minimal) ----------

type container struct {
	Name string `json:"name"`
}

type podSpec struct {
	Containers []container `json:"containers"`
}

type podStatus struct {
	Phase             string `json:"phase"`
	ContainerStatuses []struct {
		Name  string `json:"name"`
		Ready bool   `json:"ready"`
	} `json:"containerStatuses"`
}

type pod struct {
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec   podSpec   `json:"spec"`
	Status podStatus `json:"status"`
}

type podList struct {
	Items []pod `json:"items"`
}

// job mirrors batch/v1 Job (the subset we render).
type job struct {
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		// Job controller sets these; useful for display.
		BackoffLimit int32  `json:"backoffLimit"`
		Completions  *int32 `json:"completions"`
		Parallelism  *int32 `json:"parallelism"`
	} `json:"spec"`
	Status struct {
		Succeeded      int32          `json:"succeeded"`
		Failed         int32          `json:"failed"`
		Active         int32          `json:"active"`
		StartTime      *string        `json:"startTime"`
		CompletionTime *string        `json:"completionTime"`
		Conditions     []jobCondition `json:"conditions"`
	} `json:"status"`
}

type jobCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type jobList struct {
	Items []job `json:"items"`
}

type deploymentSpec struct {
	Replicas int32 `json:"replicas"`
	Selector struct {
		MatchLabels map[string]string `json:"matchLabels"`
	} `json:"selector"`
	Template struct {
		Spec podSpec `json:"spec"`
	} `json:"template"`
}

type deploymentStatus struct {
	Replicas          int32 `json:"replicas"`
	ReadyReplicas     int32 `json:"readyReplicas"`
	AvailableReplicas int32 `json:"availableReplicas"`
}

type deployment struct {
	Metadata struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec   deploymentSpec   `json:"spec"`
	Status deploymentStatus `json:"status"`
}

type deploymentList struct {
	Items []deployment `json:"items"`
}

type nodeResource struct {
	CPU    string `json:"cpu,omitempty"`    // cores or millicores, e.g. "2", "250m"
	Memory string `json:"memory,omitempty"` // e.g. "8Gi", "1600Mi"
}

type node struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Status struct {
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		NodeInfo struct {
			KubeletVersion string `json:"kubeletVersion"`
		} `json:"nodeInfo"`
		Capacity    nodeResource `json:"capacity"`
		Allocatable nodeResource `json:"allocatable"`
		Usage       nodeResource `json:"usage"`
	} `json:"status"`
}

type nodeList struct {
	Items []node `json:"items"`
}

// nodeMetrics / nodeMetricsList mirror metrics.k8s.io/v1beta1 NodeMetrics.
// CPU usage is reported in nanocores (e.g. "72213343n"); memory in Ki.
type nodeMetrics struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Usage struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	} `json:"usage"`
}

type nodeMetricsList struct {
	Items []nodeMetrics `json:"items"`
}

// ---------- views ----------

// isValidWorkloadURL checks that a workload-url annotation value is a valid URL:
//   - begins with "http://", "https://", or "/" (absolute subpath)
//   - contains only a relative or absolute subpath (no query params, no fragments
//     that would break the path, no unexpected characters)
func isValidWorkloadURL(s string) bool {
	if s == "" {
		return false
	}
	isHTTP := strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
	isAbsPath := strings.HasPrefix(s, "/")
	isRelPath := !isHTTP && !isAbsPath && strings.HasSuffix(s, "/") && len(s) > 1 && !strings.Contains(s, "//")
	// Accepted forms:
	//   - http(s)://host/path
	//   - absolute path  (/apps/echo, /vscode)
	//   - relative subpath ending in "/"  (vscode/ , /apps/terminal/)
	// A bare relative subpath must not contain "//" and must be more than "/".
	if !isHTTP && !isAbsPath && !isRelPath {
		return false
	}
	// For absolute paths, must have more than just "/"
	if isAbsPath && (s == "/" || len(s) == 1) {
		return false
	}
	// For http(s) URLs, must have a non-empty path after the scheme
	if isHTTP {
		if i := strings.Index(s, "/"); i >= 0 {
			if s[i:] == "/" {
				return false
			}
		} else {
			return false
		}
	}
	// Reject values with spaces, control chars, or obviously invalid content
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c < 0x20 {
			return false
		}
	}
	return true
}

// isValidOcticonName reports whether s is a valid Primer Octicon name.
// Octicon names are strictly lowercase alphanumerics separated by single
// hyphens (e.g. "rocket", "cloud-offline", "issue-reopened"). This matches
// the `octicon-<name>` CSS/SVG ids used in the web sprite. The icon must also
// be present in the web sprite (see web/index.html) to actually render; this
// check only guards against malformed names so a bad annotation never
// produces a broken <use href>. Note: Octicons have no case- or
// underscore-variants; the 16px name is the canonical name.
func isValidOcticonName(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for i, c := range s {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			continue
		}
		if c == '-' {
			if i > 0 && s[i-1] == '-' {
				return false // no consecutive hyphens
			}
			continue
		}
		return false // anything else (uppercase, underscore, space, etc.)
	}
	return true
}

type deploymentView struct {
	Name           string   `json:"name"`
	Namespace      string   `json:"namespace"`
	Replicas       int32    `json:"replicas"`
	Ready          int32    `json:"ready"`
	Containers     int      `json:"containers"`
	Running        bool     `json:"running"`
	ContainerNames []string `json:"containerNames"`
	WorkloadURL    string   `json:"workloadUrl,omitempty"`
	WorkloadIcon   string   `json:"workloadIcon,omitempty"`
}

type podView struct {
	Name       string   `json:"name"`
	Ready      bool     `json:"ready"`
	Containers []string `json:"containers"`
}

// ---------- handlers ----------

func (c *k8sClient) listDeployments(ctx context.Context) ([]deploymentView, error) {
	var dl deploymentList
	if err := c.do(ctx, "GET", "/apis/apps/v1/namespaces/"+*namespace+"/deployments", nil, &dl); err != nil {
		return nil, err
	}
	out := make([]deploymentView, 0, len(dl.Items))
	for _, d := range dl.Items {
		// Per spec, only deployments that carry a valid workload-url
		// annotation appear in the Deployments tab. Everything else (e.g.
		// the dashboard's own deployment) is excluded.
		u, ok := d.Metadata.Annotations["workload-url"]
		if !ok || !isValidWorkloadURL(u) {
			continue
		}

		// Optional workload-icon: a Primer Octicon name shown next to the
		// deployment name in the UI. Absent or invalid -> no icon.
		icon := ""
		if v, ok := d.Metadata.Annotations["workload-icon"]; ok && isValidOcticonName(v) {
			icon = v
		}

		names := []string{}
		for _, ctr := range d.Spec.Template.Spec.Containers {
			names = append(names, ctr.Name)
		}
		running := d.Spec.Replicas > 0 && d.Status.ReadyReplicas == d.Spec.Replicas

		out = append(out, deploymentView{
			Name: d.Metadata.Name, Namespace: d.Metadata.Namespace,
			Replicas: d.Spec.Replicas, Ready: d.Status.ReadyReplicas,
			Containers: len(names), Running: running, ContainerNames: names,
			WorkloadURL: u, WorkloadIcon: icon,
		})
	}
	return out, nil
}

// listPodsByLabels lists pods in the watched namespace matching a label
// selector. Shared by deployment and job views.
func (c *k8sClient) listPodsByLabels(ctx context.Context, selector string) ([]podView, error) {
	path := "/api/v1/namespaces/" + *namespace + "/pods?labelSelector=" + url.QueryEscape(selector)
	var pl podList
	if err := c.do(ctx, "GET", path, nil, &pl); err != nil {
		return nil, err
	}
	out := make([]podView, 0, len(pl.Items))
	for _, p := range pl.Items {
		ready := p.Status.Phase == "Running"
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Ready {
				ready = true
			}
		}
		cn := []string{}
		for _, ctr := range p.Spec.Containers {
			cn = append(cn, ctr.Name)
		}
		out = append(out, podView{Name: p.Metadata.Name, Ready: ready, Containers: cn})
	}
	return out, nil
}

func (c *k8sClient) listPodsForDeployment(ctx context.Context, name string) ([]podView, error) {
	var d deployment
	if err := c.do(ctx, "GET", "/apis/apps/v1/namespaces/"+*namespace+"/deployments/"+name, nil, &d); err != nil {
		return nil, err
	}
	parts := []string{}
	for k, v := range d.Spec.Selector.MatchLabels {
		parts = append(parts, k+"="+v)
	}
	return c.listPodsByLabels(ctx, strings.Join(parts, ","))
}

// jobView is the serialized shape for /api/jobs.
type jobView struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"` // succeeded|pending|running|failed
	Succeeded int32  `json:"succeeded"`
	Failed    int32  `json:"failed"`
	Active    int32  `json:"active"`
	// Pod to stream logs for (the most relevant job pod), and its container.
	Pod        string   `json:"pod,omitempty"`
	Container  string   `json:"container,omitempty"`
	Containers []string `json:"containers,omitempty"`
	StartTime  string   `json:"startTime,omitempty"`
	EndTime    string   `json:"endTime,omitempty"`
}

// jobStatusOf derives a UI status from a Job: succeeded, pending, running,
// or failed. Matches the requested color mapping:
//
//	succeeded -> green, pending -> yellow, running -> blue, failed -> red.
func jobStatusOf(j job) string {
	for _, c := range j.Status.Conditions {
		switch c.Type {
		case "Complete":
			if c.Status == "True" {
				return "succeeded"
			}
		case "Failed":
			if c.Status == "True" {
				return "failed"
			}
		}
	}
	// No terminal condition yet.
	if j.Status.Active > 0 {
		return "running"
	}
	if j.Status.Failed > 0 && j.Status.Succeeded == 0 {
		return "failed"
	}
	if j.Status.Succeeded > 0 && j.Status.Failed == 0 {
		return "succeeded"
	}
	return "pending"
}

func (c *k8sClient) listJobs(ctx context.Context) ([]jobView, error) {
	var jl jobList
	if err := c.do(ctx, "GET", "/apis/batch/v1/namespaces/"+*namespace+"/jobs", nil, &jl); err != nil {
		return nil, err
	}
	out := make([]jobView, 0, len(jl.Items))
	for _, j := range jl.Items {
		v := jobView{
			Name:      j.Metadata.Name,
			Namespace: j.Metadata.Namespace,
			Status:    jobStatusOf(j),
			Succeeded: j.Status.Succeeded,
			Failed:    j.Status.Failed,
			Active:    j.Status.Active,
		}
		if j.Status.StartTime != nil {
			v.StartTime = *j.Status.StartTime
		}
		if j.Status.CompletionTime != nil {
			v.EndTime = *j.Status.CompletionTime
		}

		// Find the job's pod(s) to stream output from. Job pods carry the
		// label batch.kubernetes.io/job-name=<jobName>.
		pods, err := c.listPodsByLabels(ctx, "batch.kubernetes.io/job-name="+url.QueryEscape(j.Metadata.Name))
		if err == nil && len(pods) > 0 {
			// Prefer a pod that has actually run or is running.
			pick := pods[0]
			for _, p := range pods {
				if p.Ready {
					pick = p
					break
				}
			}
			v.Pod = pick.Name
			if len(pick.Containers) > 0 {
				v.Container = pick.Containers[0]
			}
			v.Containers = pick.Containers
		}
		out = append(out, v)
	}
	// Stable order: name.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *k8sClient) handleJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := c.listJobs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (c *k8sClient) scaleDeployment(ctx context.Context, name string, replicas int32) error {
	var d deployment
	if err := c.do(ctx, "GET", "/apis/apps/v1/namespaces/"+*namespace+"/deployments/"+name, nil, &d); err != nil {
		return err
	}
	// Remember the original replica count when scaling to 0.
	if replicas == 0 {
		if d.Metadata.Annotations == nil {
			d.Metadata.Annotations = map[string]string{}
		}
		if _, ok := d.Metadata.Annotations["deploy-dashboard/last-replicas"]; !ok {
			d.Metadata.Annotations["deploy-dashboard/last-replicas"] = strconv.Itoa(int(d.Spec.Replicas))
		}
	} else if d.Metadata.Annotations != nil {
		delete(d.Metadata.Annotations, "deploy-dashboard/last-replicas")
	}
	d.Spec.Replicas = replicas

	// Use the strategic merge patch endpoint for safety.
	patch := map[string]any{"spec": map[string]any{"replicas": replicas}}
	patchBody, _ := json.Marshal(patch)
	return c.do(ctx, "PATCH", "/apis/apps/v1/namespaces/"+*namespace+"/deployments/"+name, bytes.NewReader(patchBody), nil)
}

func (c *k8sClient) deletePods(ctx context.Context, name string) error {
	pods, err := c.listPodsForDeployment(ctx, name)
	if err != nil {
		return err
	}
	var firstErr error
	for _, p := range pods {
		err := c.do(ctx, "DELETE", "/api/v1/namespaces/"+*namespace+"/pods/"+p.Name, nil, nil)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *k8sClient) deleteDeployment(ctx context.Context, name string) error {
	return c.do(ctx, "DELETE", "/apis/apps/v1/namespaces/"+*namespace+"/deployments/"+name, nil, nil)
}

// ── User Settings (Secret) ──────────────────────────────────────
//
// Sensitive user data (SSH public key, access tokens) is stored in a
// namespaced Secret named "user-settings", not a ConfigMap: Secrets are the
// correct primitive for credentials and can be encrypted at rest, whereas
// ConfigMaps are readable by anyone with namespace get and show up in
// kubectl dumps and etcd snapshots.
//
// The backend is the only thing that reads/writes the Secret; the API surface
// (/api/user-settings, /api/tokens) is unchanged, so the frontend is untouched.
// The RBAC grant is scoped to this single Secret name (see 02-rbac.yaml).

const userSettingsSecretName = "user-settings"

func (c *k8sClient) getUserSettings(ctx context.Context) (map[string]string, error) {
	var sec struct {
		StringData map[string]string `json:"stringData"`
		Data       map[string]string `json:"data"` // base64 (stringData takes precedence)
	}
	path := "/api/v1/namespaces/" + *namespace + "/secrets/" + userSettingsSecretName
	if err := c.do(ctx, "GET", path, nil, &sec); err != nil {
		if strings.Contains(err.Error(), "404") {
			return map[string]string{}, nil
		}
		return nil, err
	}
	out := map[string]string{}
	// "data" holds base64 values; decode into the map.
	for k, v := range sec.Data {
		decoded, derr := base64.StdEncoding.DecodeString(v)
		if derr != nil {
			// Skip undecodable entries rather than failing the whole read.
			continue
		}
		out[k] = string(decoded)
	}
	// "stringData" (plain) overrides any decoded "data" for the same key.
	for k, v := range sec.StringData {
		out[k] = v
	}
	return out, nil
}

// upsertUserSettings creates-or-updates the user-settings Secret. We write
// with stringData (plain values) so the API server base64-encodes for us and
// we never have to hand-encode.
func (c *k8sClient) upsertUserSettings(ctx context.Context, data map[string]string) error {
	body := map[string]any{
		"metadata":   map[string]any{"name": userSettingsSecretName, "namespace": *namespace},
		"type":       "Opaque",
		"stringData": data,
	}
	b, _ := json.Marshal(body)
	// Try to create first; if it already exists, fall back to PUT.
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/v1/namespaces/"+*namespace+"/secrets", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == 201 {
		return nil
	}
	return c.putUserSettings(ctx, data)
}

func (c *k8sClient) putUserSettings(ctx context.Context, data map[string]string) error {
	body := map[string]any{
		"metadata":   map[string]any{"name": userSettingsSecretName, "namespace": *namespace},
		"type":       "Opaque",
		"stringData": data,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "PUT",
		c.baseURL+"/api/v1/namespaces/"+*namespace+"/secrets/"+userSettingsSecretName, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT secret %s: %s %s", userSettingsSecretName, resp.Status, string(respBody))
	}
	return nil
}

func (c *k8sClient) handleUserSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		data, err := c.getUserSettings(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	case "PUT":
		var data map[string]string
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Merge with existing
		existing, _ := c.getUserSettings(r.Context())
		if existing == nil {
			existing = map[string]string{}
		}
		for k, v := range data {
			if v == "" {
				delete(existing, k)
			} else {
				existing[k] = v
			}
		}
		if err := c.upsertUserSettings(r.Context(), existing); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *k8sClient) handleTokens(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		data, err := c.getUserSettings(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Return only token entries (keys starting with "token.")
		tokens := map[string]string{}
		for k, v := range data {
			if strings.HasPrefix(k, "token.") {
				tokens[k] = v
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokens)
	case "PUT":
		var tokens map[string]string
		if err := json.NewDecoder(r.Body).Decode(&tokens); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Merge into user-settings with "token." prefix
		data := map[string]string{}
		for k, v := range tokens {
			data["token."+k] = v
		}
		// Also preserve non-token keys
		existing, _ := c.getUserSettings(r.Context())
		if existing != nil {
			for k, v := range existing {
				if !strings.HasPrefix(k, "token.") {
					data[k] = v
				}
			}
		}
		// Remove old tokens not in the new set
		for k := range existing {
			if strings.HasPrefix(k, "token.") {
				base := strings.TrimPrefix(k, "token.")
				if _, ok := tokens[base]; !ok {
					delete(data, k)
				}
			}
		}
		if err := c.upsertUserSettings(r.Context(), data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *k8sClient) streamLogs(w http.ResponseWriter, r *http.Request, podName, containerName string) {
	// The k8s API streaming endpoint requires a long-lived connection.
	// We open a raw HTTP request (not through c.do) so we control the lifecycle.
	req, err := http.NewRequestWithContext(r.Context(), "GET",
		c.baseURL+"/api/v1/namespaces/"+*namespace+"/pods/"+podName+"/log?follow=true&tailLines=200", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if containerName != "" {
		req.URL.Query().Set("container", containerName)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	// Use a client with no timeout for streaming.
	streamClient := &http.Client{
		Transport: c.httpClient.Transport,
		Timeout:   0, // no timeout for log streaming
	}
	resp, err := streamClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		http.Error(w, string(b), resp.StatusCode)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	buf := make([]byte, 32*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flusher.Flush()
		}
		if rerr != nil {
			if rerr != io.EOF {
				fmt.Fprintf(w, "\n[stream ended: %v]\n", rerr)
				flusher.Flush()
			}
			return
		}
	}
}

func (c *k8sClient) handleDeployments(w http.ResponseWriter, r *http.Request) {
	dps, err := c.listDeployments(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dps)
}

func (c *k8sClient) handlePods(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("deployment")
	if name == "" {
		http.Error(w, "deployment param required", http.StatusBadRequest)
		return
	}
	pods, err := c.listPodsForDeployment(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pods)
}

func (c *k8sClient) handleScale(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("deployment")
	if name == "" {
		http.Error(w, "deployment param required", http.StatusBadRequest)
		return
	}
	var req struct {
		Replicas int32 `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	if err := c.scaleDeployment(r.Context(), name, req.Replicas); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "deployment": name, "replicas": req.Replicas})
}

func (c *k8sClient) handleKillPods(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("deployment")
	if name == "" {
		http.Error(w, "deployment param required", http.StatusBadRequest)
		return
	}
	if err := c.deletePods(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "deployment": name, "podsDeleted": true})
}

func (c *k8sClient) handleDeleteDeployment(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("deployment")
	if name == "" {
		http.Error(w, "deployment param required", http.StatusBadRequest)
		return
	}
	if err := c.deleteDeployment(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "deployment": name, "deleted": true})
}

func (c *k8sClient) handleLogs(w http.ResponseWriter, r *http.Request) {
	pod := r.URL.Query().Get("pod")
	container := r.URL.Query().Get("container")
	if pod == "" {
		http.Error(w, "pod param required", http.StatusBadRequest)
		return
	}
	c.streamLogs(w, r, pod, container)
}

// parseCPUQuantity converts a Kubernetes CPU quantity ("2", "250m", "0.5")
// to a float number of cores. Returns -1 on parse failure.
func parseCPUQuantity(q string) float64 {
	q = strings.TrimSpace(q)
	if q == "" {
		return -1
	}
	if strings.HasSuffix(q, "m") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(q, "m"), 64)
		if err != nil {
			return -1
		}
		return v / 1000
	}
	v, err := strconv.ParseFloat(q, 64)
	if err != nil {
		return -1
	}
	return v
}

// parseMemQuantity converts a Kubernetes memory quantity ("8Gi", "1600Mi",
// "1G", "512M", "1000") to a float number of bytes. Returns -1 on failure.
func parseMemQuantity(q string) float64 {
	q = strings.TrimSpace(q)
	if q == "" {
		return -1
	}
	// Find the leading numeric part (may include a decimal point).
	i := 0
	for i < len(q) && (q[i] >= '0' && q[i] <= '9' || q[i] == '.') {
		i++
	}
	num, err := strconv.ParseFloat(q[:i], 64)
	if err != nil {
		return -1
	}
	unit := q[i:]
	switch unit {
	case "":
		return num
	case "Ki":
		return num * 1024
	case "Mi":
		return num * 1024 * 1024
	case "Gi":
		return num * 1024 * 1024 * 1024
	case "Ti":
		return num * 1024 * 1024 * 1024 * 1024
	case "Pi":
		return num * 1024 * 1024 * 1024 * 1024 * 1024
	case "K":
		return num * 1e3
	case "M":
		return num * 1e6
	case "G":
		return num * 1e9
	case "T":
		return num * 1e12
	case "P":
		return num * 1e15
	default:
		return -1
	}
}

// fmtMilliCores renders a millicore count for display (e.g. "1250m", "2.5k").
func fmtMilliCores(m float64) string {
	if m < 0 {
		return "—"
	}
	if m >= 1000 {
		return fmt.Sprintf("%.1fk", m/1000)
	}
	return fmt.Sprintf("%.0fm", m)
}

// fmtBytes renders a byte count in human form (e.g. "8 Gi", "1.5 Mi").
func fmtBytes(b float64) string {
	if b < 0 {
		return "—"
	}
	units := []string{"B", "Ki", "Mi", "Gi", "Ti", "Pi"}
	v := b
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if v >= 100 || i == 0 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

// parseNanoCPU converts a nanocore quantity ("72213343n", "100m", "1") to
// millicores, as reported by the metrics server. Returns -1 on failure.
func parseNanoCPU(q string) float64 {
	q = strings.TrimSpace(q)
	if q == "" {
		return -1
	}
	switch {
	case strings.HasSuffix(q, "n"):
		v, err := strconv.ParseFloat(strings.TrimSuffix(q, "n"), 64)
		if err != nil {
			return -1
		}
		return v / 1e6
	case strings.HasSuffix(q, "u"):
		v, err := strconv.ParseFloat(strings.TrimSuffix(q, "u"), 64)
		if err != nil {
			return -1
		}
		return v / 1e3
	case strings.HasSuffix(q, "m"):
		v, err := strconv.ParseFloat(strings.TrimSuffix(q, "m"), 64)
		if err != nil {
			return -1
		}
		return v
	default:
		v, err := strconv.ParseFloat(q, 64)
		if err != nil {
			return -1
		}
		return v * 1000
	}
}

func (c *k8sClient) handleNodes(w http.ResponseWriter, r *http.Request) {
	var nl nodeList
	if err := c.do(r.Context(), "GET", "/api/v1/nodes", nil, &nl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Per-node usage from the metrics server. Non-fatal: if the metrics API is
	// absent or forbidden, usage shows "—" while totals still render.
	usage := map[string]nodeMetrics{}
	var nml nodeMetricsList
	if err := c.do(r.Context(), "GET", "/apis/metrics.k8s.io/v1beta1/nodes", nil, &nml); err == nil {
		for _, m := range nml.Items {
			usage[m.Metadata.Name] = m
		}
	}

	out := []map[string]any{}
	for _, n := range nl.Items {
		roles := []string{}
		for k, v := range n.Metadata.Labels {
			// Role labels may be present with an empty value (""/true) or
			// "true" (k3s). Accept both so control-plane/etcd are detected.
			if strings.HasPrefix(k, "node-role.kubernetes.io/") && (v == "" || v == "true") {
				roles = append(roles, strings.TrimPrefix(k, "node-role.kubernetes.io/"))
			}
		}
		// Sort for stable display (map iteration is random).
		sort.Strings(roles)
		if len(roles) == 0 {
			roles = append(roles, "worker")
		}
		// Map the Ready condition to a clean UI state. "True" -> Ready,
		// "False" -> NotReady, anything else (or missing) -> Unknown.
		ready := "Unknown"
		for _, cond := range n.Status.Conditions {
			if cond.Type == "Ready" {
				switch cond.Status {
				case "True":
					ready = "Ready"
				case "False":
					ready = "NotReady"
				default:
					ready = "Unknown"
				}
			}
		}
		// Totals from the Node's allocatable; usage from metrics (if present).
		cpuTotal := parseCPUQuantity(n.Status.Allocatable.CPU) * 1000 // cores -> millicores
		memTotal := parseMemQuantity(n.Status.Allocatable.Memory)     // bytes
		var cpuUsed, memUsed float64 = -1, -1
		if m, ok := usage[n.Metadata.Name]; ok {
			cpuUsed = parseNanoCPU(m.Usage.CPU)        // millicores
			memUsed = parseMemQuantity(m.Usage.Memory) // bytes
		}
		out = append(out, map[string]any{
			"name": n.Metadata.Name, "status": ready,
			"roles":   strings.Join(roles, ","),
			"version": n.Status.NodeInfo.KubeletVersion,
			// Raw quantity strings (for tooltips / future detail views).
			"capacity":    map[string]any{"cpu": n.Status.Capacity.CPU, "memory": n.Status.Capacity.Memory},
			"allocatable": map[string]any{"cpu": n.Status.Allocatable.CPU, "memory": n.Status.Allocatable.Memory},
			// Parsed numbers the UI plots (usage is -1 if metrics unavailable).
			"cpuUsedM":     cpuUsed,
			"cpuTotalM":    cpuTotal,
			"memUsedB":     memUsed,
			"memTotalB":    memTotal,
			"cpuUsedDisp":  fmtMilliCores(cpuUsed),
			"cpuTotalDisp": fmtMilliCores(cpuTotal),
			"memUsedDisp":  fmtBytes(memUsed),
			"memTotalDisp": fmtBytes(memTotal),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (c *k8sClient) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// webFS holds the single-page app, embedded at build time so the binary is
// self-contained. It is the single source of truth for the UI: edit
// web/index.html and rebuild -- no codegen or env var required. The actual
// //go:embed directive lives at the module root (embed.go) because Go cannot
// embed files outside the declaring package's directory tree.
var webFS = deploydashboard.WebFS

func (c *k8sClient) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Serve the UI embedded at build time (see webFS above).
	b, err := webFS.ReadFile("web/index.html")
	if err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
		return
	}
	// Fallback: try to read from disk (for local dev, when running outside
	// the module tree where //go:embed could not pick the file up).
	b, err = os.ReadFile(filepath.Join("web", "index.html"))
	if err != nil {
		http.Error(w, "index.html not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}

// ---------- WebSocket log streaming ----------

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// handleLogsWS handles WebSocket upgrade on the same HTTP server.
// It uses the original bufio.ReadWriter from Hijack() for all I/O,
// which has the correct read position after the HTTP request headers.
func (c *k8sClient) handleLogsWS(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") != "websocket" ||
		!strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		http.Error(w, "not a websocket upgrade", http.StatusBadRequest)
		return
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Drain the bufio.Reader's buffer. After Hijack(), the bufio.Reader
	// may have buffered data from the client (e.g., the first WebSocket
	// frame if sent in the same TCP packet as the HTTP request).
	// We need to re-feed this data to the frame reader.
	var leftover []byte
	if n := rw.Reader.Buffered(); n > 0 {
		leftover = make([]byte, n)
		if _, err := rw.Reader.Read(leftover); err != nil {
			log.Printf("ws: failed to drain buffer: %v", err)
			leftover = nil
		} else {
			log.Printf("ws: drained %d bytes from bufio buffer", n)
		}
	}

	// Write the 101 response directly to the conn (bypassing bufio).
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := conn.Write([]byte(resp)); err != nil {
		return
	}
	log.Printf("ws: upgraded connection from %s", r.RemoteAddr)

	// Writer goroutine: serializes all writes using conn directly
	writeCh := make(chan []byte, 256)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for data := range writeCh {
			if wsWriteFrameConn(conn, 0x1, data) != nil {
				return
			}
		}
	}()
	wsSend := func(data []byte) {
		select {
		case writeCh <- data:
		default:
		}
	}

	// Reader goroutine: reads client frames using a combined reader that
	// first serves the drained bytes, then reads from the conn.
	cmdCh := make(chan map[string]string, 16)
	errCh := make(chan error, 1)
	combinedReader := &wsCombinedReader{data: leftover, conn: conn}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic: %v", r)
			}
		}()
		for {
			opcode, payload, readErr := wsReadFrameReader(combinedReader)
			if readErr != nil {
				errCh <- readErr
				return
			}
			if opcode == 0x8 { // close
				errCh <- io.EOF
				return
			}
			if opcode == 0x9 { // ping
				_ = wsWriteFrame(rw, conn, 0xA, payload)
				continue
			}
			if opcode == 0x1 { // text
				var cmd map[string]string
				if jsonErr := json.Unmarshal(payload, &cmd); jsonErr == nil {
					cmdCh <- cmd
				}
			}
		}
	}()

	// Log stream management
	type logStream struct {
		cancel context.CancelFunc
		done   chan struct{}
	}
	streams := map[string]*logStream{}

	for {
		select {
		case cmd := <-cmdCh:
			switch cmd["action"] {
			case "follow":
				pod := cmd["pod"]
				container := cmd["container"]
				if pod == "" {
					wsSend([]byte(`{"error":"pod required"}`))
					continue
				}
				if old, ok := streams[pod]; ok {
					old.cancel()
					<-old.done
					delete(streams, pod)
				}
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan struct{})
				streams[pod] = &logStream{cancel: cancel, done: done}

				go func(pod, container string, done chan struct{}, cancel context.CancelFunc) {
					defer close(done)
					defer cancel()
					req, reqErr := http.NewRequestWithContext(ctx, "GET",
						c.baseURL+"/api/v1/namespaces/"+*namespace+"/pods/"+pod+"/log?follow=true&tailLines=200", nil)
					if reqErr != nil {
						wsSend([]byte(`{"error":"` + reqErr.Error() + `"}`))
						return
					}
					if container != "" {
						req.URL.Query().Set("container", container)
					}
					req.Header.Set("Authorization", "Bearer "+c.token)
					streamClient := &http.Client{
						Transport: c.httpClient.Transport,
						Timeout:   0,
					}
					resp, doErr := streamClient.Do(req)
					if doErr != nil {
						wsSend([]byte(`{"error":"` + doErr.Error() + `"}`))
						return
					}
					defer resp.Body.Close()
					if resp.StatusCode != 200 {
						b, _ := io.ReadAll(resp.Body)
						wsSend([]byte(`{"error":"` + string(b) + `"}`))
						return
					}
					wsSend([]byte(`{"start":true,"pod":"` + pod + `"}`))
					buf := make([]byte, 32*1024)
					for {
						n, rerr := resp.Body.Read(buf)
						if n > 0 {
							wsSend(append([]byte{}, buf[:n]...))
						}
						if rerr != nil {
							if rerr != io.EOF {
								wsSend([]byte(`{"end":true,"error":"` + rerr.Error() + `"}`))
							} else {
								wsSend([]byte(`{"end":true}`))
							}
							return
						}
					}
				}(pod, container, done, cancel)

			case "stop":
				pod := cmd["pod"]
				if pod == "" {
					for p, s := range streams {
						s.cancel()
						<-s.done
						delete(streams, p)
					}
				} else if s, ok := streams[pod]; ok {
					s.cancel()
					<-s.done
					delete(streams, pod)
				}
			}

		case readErr := <-errCh:
			if readErr != nil && readErr != io.EOF {
				wsSend([]byte(`{"error":"` + readErr.Error() + `"}`))
			}
			close(writeCh)
			<-writeDone
			return
		}
	}
}

func wsWriteFrame(rw *bufio.ReadWriter, conn net.Conn, opcode byte, data []byte) error {
	var header []byte
	if len(data) < 126 {
		header = []byte{0x80 | opcode, byte(len(data))}
	} else if len(data) < 65536 {
		header = []byte{0x80 | opcode, 126, byte(len(data) >> 8), byte(len(data))}
	} else {
		header = []byte{0x80 | opcode, 127}
		var lenBuf [8]byte
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(data)))
		header = append(header, lenBuf[:]...)
	}
	if _, err := rw.Write(append(header, data...)); err != nil {
		return err
	}
	return rw.Flush()
}

func wsReadFrame(rw *bufio.ReadWriter) (byte, []byte, error) {
	head := make([]byte, 2)
	if _, err := io.ReadFull(rw, head); err != nil {
		return 0, nil, err
	}
	opcode := head[0] & 0x0f
	masked := head[1]&0x80 != 0
	var length uint64
	switch head[1] & 0x7f {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(rw, ext); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(rw, ext); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext)
	default:
		length = uint64(head[1] & 0x7f)
	}
	if length > 1024*1024 {
		return 0, nil, fmt.Errorf("frame too large: %d", length)
	}
	var mask []byte
	if masked {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(rw, mask); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(rw, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, nil
}

// wsWriteFrameConn writes a WebSocket frame directly to the conn.
func wsWriteFrameConn(conn net.Conn, opcode byte, data []byte) error {
	var header []byte
	if len(data) < 126 {
		header = []byte{0x80 | opcode, byte(len(data))}
	} else if len(data) < 65536 {
		header = []byte{0x80 | opcode, 126, byte(len(data) >> 8), byte(len(data))}
	} else {
		header = []byte{0x80 | opcode, 127}
		var lenBuf [8]byte
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(data)))
		header = append(header, lenBuf[:]...)
	}
	_, err := conn.Write(append(header, data...))
	return err
}

// wsCombinedReader first serves buffered data, then reads from the conn.
type wsCombinedReader struct {
	data []byte
	conn net.Conn
}

func (r *wsCombinedReader) Read(p []byte) (int, error) {
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	return r.conn.Read(p)
}

// wsReadFrameReader reads a WebSocket frame from a generic io.Reader.
func wsReadFrameReader(r io.Reader) (byte, []byte, error) {
	head := make([]byte, 2)
	if _, err := io.ReadFull(r, head); err != nil {
		return 0, nil, err
	}
	opcode := head[0] & 0x0f
	masked := head[1]&0x80 != 0
	var length uint64
	switch head[1] & 0x7f {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext)
	default:
		length = uint64(head[1] & 0x7f)
	}
	if length > 1024*1024 {
		return 0, nil, fmt.Errorf("frame too large: %d", length)
	}
	var mask []byte
	if masked {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(r, mask); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, nil
}

// basePathHandler serves the app (mux) under an optional base path. With an
// empty base it is a no-op passthrough. With a base (e.g. "/app") it lets the
// same SPA + API work at /app/ and /app/api/... by stripping the base for the
// API/healthz routes and serving index.html for the base path itself. The SPA
// uses relative api/* calls, so this is what makes it subpath-aware.
func basePathHandler(mux http.Handler, base string) http.Handler {
	base = strings.TrimSuffix(base, "/")
	if base == "" {
		return mux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		// Exact base path (e.g. /app or /app/) -> serve the SPA.
		if p == base || p == base+"/" {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			r2.RequestURI = "/"
			mux.ServeHTTP(w, r2)
			return
		}
		// Everything under the base -> strip the base and re-dispatch.
		if strings.HasPrefix(p, base+"/") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/" + strings.TrimPrefix(p, base+"/")
			r2.RequestURI = r2.URL.RequestURI()
			mux.ServeHTTP(w, r2)
			return
		}
		// Outside the base -> 404 (the gateway only routes the base here anyway).
		http.NotFound(w, r)
	})
}

func main() {
	flag.Parse()

	c, err := newK8sClient()
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", c.handleIndex)
	mux.HandleFunc("/healthz", c.handleHealthz)
	mux.HandleFunc("/api/deployments", c.handleDeployments)
	mux.HandleFunc("/api/jobs", c.handleJobs)
	mux.HandleFunc("/api/pods", c.handlePods)
	mux.HandleFunc("/api/scale", c.handleScale)
	mux.HandleFunc("/api/kill-pods", c.handleKillPods)
	mux.HandleFunc("/api/delete-deployment", c.handleDeleteDeployment)
	mux.HandleFunc("/api/logs", c.handleLogs)
	mux.HandleFunc("/api/logs-ws", c.handleLogsWS)
	mux.HandleFunc("/api/nodes", c.handleNodes)
	mux.HandleFunc("/api/user-settings", c.handleUserSettings)
	mux.HandleFunc("/api/tokens", c.handleTokens)

	srv := &http.Server{
		Addr:              *listenAddr,
		Handler:           basePathHandler(mux, *basePath),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("deploy-dashboard listening on %s (namespace=%s)", *listenAddr, *namespace)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
	_ = bufio.Scanner{} // keep import
}
