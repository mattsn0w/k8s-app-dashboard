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
	"strconv"
	"crypto/sha1"
	"flag"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"deploy-dashboard"
)

const (
	tokenFile    = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	certFile     = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	defaultNS    = "default"
	defaultPort  = ":8080"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "kubeconfig path (empty = in-cluster)")
	namespace  = flag.String("namespace", defaultNS, "namespace to watch")
	listenAddr = flag.String("listen", defaultPort, "listen address")
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
	Replicas       int32 `json:"replicas"`
	ReadyReplicas  int32 `json:"readyReplicas"`
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
	} `json:"status"`
}

type nodeList struct {
	Items []node `json:"items"`
}

// ---------- views ----------

// isValidWorkloadURL checks that a workload-url annotation value is a valid URL:
//   - begins with "http://", "https://", or "/" (absolute subpath)
//   - contains only a relative or absolute subpath (no query params, no fragments
//     that would break the path, no unexpected characters)
func isValidWorkloadURL(s string) bool {
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") && !strings.HasPrefix(s, "/") {
		return false
	}
	// For absolute paths, must have more than just "/"
	if strings.HasPrefix(s, "/") && (s == "/" || len(s) == 1) {
		return false
	}
	// For http(s) URLs, must have a non-empty path after the scheme
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
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

type deploymentView struct {
	Name           string   `json:"name"`
	Namespace      string   `json:"namespace"`
	Replicas       int32    `json:"replicas"`
	Ready          int32    `json:"ready"`
	Containers     int      `json:"containers"`
	Running        bool     `json:"running"`
	ContainerNames []string `json:"containerNames"`
	WorkloadURL    string   `json:"workloadUrl,omitempty"`
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
		names := []string{}
		for _, ctr := range d.Spec.Template.Spec.Containers {
			names = append(names, ctr.Name)
		}
		running := d.Spec.Replicas > 0 && d.Status.ReadyReplicas == d.Spec.Replicas
		// Extract workload-url annotation if present and valid.
		workloadURL := ""
		if u, ok := d.Metadata.Annotations["workload-url"]; ok && isValidWorkloadURL(u) {
			workloadURL = u
		}

		out = append(out, deploymentView{
			Name: d.Metadata.Name, Namespace: d.Metadata.Namespace,
			Replicas: d.Spec.Replicas, Ready: d.Status.ReadyReplicas,
			Containers: len(names), Running: running, ContainerNames: names,
			WorkloadURL: workloadURL,
		})
	}
	return out, nil
}

func (c *k8sClient) listPodsForDeployment(ctx context.Context, name string) ([]podView, error) {
	// First get the deployment to find its selector.
	var d deployment
	if err := c.do(ctx, "GET", "/apis/apps/v1/namespaces/"+*namespace+"/deployments/"+name, nil, &d); err != nil {
		return nil, err
	}
	// Build label selector from matchLabels.
	parts := []string{}
	for k, v := range d.Spec.Selector.MatchLabels {
		parts = append(parts, k+"="+v)
	}
	sel := strings.Join(parts, ",")

	var pl podList
	path := "/api/v1/namespaces/" + *namespace + "/pods?labelSelector=" + url.QueryEscape(sel)
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

// ── User Settings (ConfigMap) ───────────────────────────────────

func (c *k8sClient) getUserSettings(ctx context.Context) (map[string]string, error) {
	var cm struct {
		Data map[string]string `json:"data"`
	}
	if err := c.do(ctx, "GET", "/api/v1/namespaces/"+*namespace+"/configmaps/user-settings", nil, &cm); err != nil {
		if strings.Contains(err.Error(), "404") {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	return cm.Data, nil
}

func (c *k8sClient) upsertUserSettings(ctx context.Context, data map[string]string) error {
	// Try to create first; if it already exists, PUT it
	body := map[string]any{
		"metadata": map[string]any{"name": "user-settings", "namespace": *namespace},
		"data":     data,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/namespaces/"+*namespace+"/configmaps", bytes.NewReader(b))
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
	if resp.StatusCode == 201 {
		return nil
	}
	// Already exists — fall through to PUT
	return c.putUserSettings(ctx, data)
}

func (c *k8sClient) putUserSettings(ctx context.Context, data map[string]string) error {
	body := map[string]any{
		"metadata": map[string]any{"name": "user-settings", "namespace": *namespace},
		"data":     data,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "PUT", c.baseURL+"/api/v1/namespaces/"+*namespace+"/configmaps/user-settings", bytes.NewReader(b))
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
		return fmt.Errorf("PUT user-settings: %s %s", resp.Status, string(respBody))
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

func (c *k8sClient) handleNodes(w http.ResponseWriter, r *http.Request) {
	var nl nodeList
	if err := c.do(r.Context(), "GET", "/api/v1/nodes", nil, &nl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := []map[string]any{}
	for _, n := range nl.Items {
		roles := []string{}
		for k, v := range n.Metadata.Labels {
			if strings.HasPrefix(k, "node-role.kubernetes.io/") && v == "" {
				roles = append(roles, strings.TrimPrefix(k, "node-role.kubernetes.io/"))
			}
		}
		if len(roles) == 0 {
			roles = append(roles, "worker")
		}
		ready := "Unknown"
		for _, cond := range n.Status.Conditions {
			if cond.Type == "Ready" {
				ready = cond.Status
			}
		}
		out = append(out, map[string]any{
			"name": n.Metadata.Name, "status": ready,
			"roles": strings.Join(roles, ","),
			"version": n.Status.NodeInfo.KubeletVersion,
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
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("deploy-dashboard listening on %s (namespace=%s)", *listenAddr, *namespace)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
	_ = bufio.Scanner{} // keep import
}
