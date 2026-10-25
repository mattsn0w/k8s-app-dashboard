package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestK8sClient creates a k8sClient pointing at a test server.
func newTestK8sClient(handler http.Handler) (*k8sClient, *httptest.Server) {
	server := httptest.NewServer(handler)
	return &k8sClient{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}, server
}

// ---------- listDeployments ----------

func TestListDeployments(t *testing.T) {
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/deployments") {
			t.Errorf("path = %s, want /deployments", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}
		// Both carry a valid workload-url so they pass the list filter.
		resp := deploymentList{
			Items: []deployment{
				{
					Metadata: struct {
						Name        string            `json:"name"`
						Namespace   string            `json:"namespace"`
						Annotations map[string]string `json:"annotations"`
					}{Name: "web", Namespace: "apps", Annotations: map[string]string{"workload-url": "/web/"}},
					Spec:   deploymentSpec{Replicas: 3},
					Status: deploymentStatus{ReadyReplicas: 3, Replicas: 3},
				},
				{
					Metadata: struct {
						Name        string            `json:"name"`
						Namespace   string            `json:"namespace"`
						Annotations map[string]string `json:"annotations"`
					}{Name: "api", Namespace: "apps", Annotations: map[string]string{"workload-url": "/api/"}},
					Spec:   deploymentSpec{Replicas: 1},
					Status: deploymentStatus{ReadyReplicas: 0, Replicas: 1},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	views, err := c.listDeployments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("len = %d, want 2", len(views))
	}
	if views[0].Name != "web" {
		t.Errorf("views[0].Name = %q, want web", views[0].Name)
	}
	if views[0].Ready != 3 {
		t.Errorf("views[0].Ready = %d, want 3", views[0].Ready)
	}
	if views[1].Ready != 0 {
		t.Errorf("views[1].Ready = %d, want 0", views[1].Ready)
	}
}

func TestListDeployments_404(t *testing.T) {
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	_, err := c.listDeployments(context.Background())
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestListDeployments_401(t *testing.T) {
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer server.Close()

	_, err := c.listDeployments(context.Background())
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

// ---------- scaleDeployment ----------

func TestScaleDeployment(t *testing.T) {
	var gotMethods []string
	var gotBody string
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethods = append(gotMethods, r.Method)
		if r.Method == "GET" {
			// Return a valid deployment
			resp := deployment{
				Metadata: struct {
					Name        string            `json:"name"`
					Namespace   string            `json:"namespace"`
					Annotations map[string]string `json:"annotations"`
				}{Name: "web", Namespace: "apps"},
				Spec:   deploymentSpec{Replicas: 3},
				Status: deploymentStatus{ReadyReplicas: 3, Replicas: 3},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "PATCH" {
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			gotBody = string(buf[:n])
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		}
	}))
	defer server.Close()

	err := c.scaleDeployment(context.Background(), "web", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotMethods) != 2 {
		t.Fatalf("len(methods) = %d, want 2 (GET + PATCH)", len(gotMethods))
	}
	if gotMethods[0] != "GET" {
		t.Errorf("methods[0] = %s, want GET", gotMethods[0])
	}
	if gotMethods[1] != "PATCH" {
		t.Errorf("methods[1] = %s, want PATCH", gotMethods[1])
	}
	if !strings.Contains(gotBody, "5") {
		t.Errorf("body = %q, want contains 5", gotBody)
	}
}

func TestScaleDeployment_PatchContentType(t *testing.T) {
	var gotPatchContentType string
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			resp := deployment{
				Metadata: struct {
					Name        string            `json:"name"`
					Namespace   string            `json:"namespace"`
					Annotations map[string]string `json:"annotations"`
				}{Name: "web", Namespace: "apps"},
				Spec:   deploymentSpec{Replicas: 3},
				Status: deploymentStatus{ReadyReplicas: 3, Replicas: 3},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "PATCH" {
			gotPatchContentType = r.Header.Get("Content-Type")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		}
	}))
	defer server.Close()

	err := c.scaleDeployment(context.Background(), "web", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPatchContentType != "application/merge-patch+json" {
		t.Errorf("PATCH Content-Type = %q, want application/merge-patch+json", gotPatchContentType)
	}
}

// ---------- deletePods ----------

func TestDeletePods(t *testing.T) {
	var gotMethods []string
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethods = append(gotMethods, r.Method)
		if r.Method == "GET" {
			resp := podList{
				Items: []pod{
					{
						Metadata: struct {
							Name      string            `json:"name"`
							Namespace string            `json:"namespace"`
							Labels    map[string]string `json:"labels"`
						}{Name: "web-abc123", Namespace: "apps", Labels: map[string]string{"app": "web"}},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "DELETE" {
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		}
	}))
	defer server.Close()

	err := c.deletePods(context.Background(), "web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// listPodsForDeployment does 2 GETs (deployment + pods), then 1 DELETE
	if len(gotMethods) != 3 {
		t.Fatalf("len(methods) = %d, want 3 (2 GETs + DELETE)", len(gotMethods))
	}
	if gotMethods[0] != "GET" {
		t.Errorf("methods[0] = %s, want GET", gotMethods[0])
	}
	if gotMethods[1] != "GET" {
		t.Errorf("methods[1] = %s, want GET", gotMethods[1])
	}
	if gotMethods[2] != "DELETE" {
		t.Errorf("methods[2] = %s, want DELETE", gotMethods[2])
	}
}

// ---------- deleteDeployment ----------

func TestDeleteDeployment(t *testing.T) {
	var gotMethod, gotPath string
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer server.Close()

	err := c.deleteDeployment(context.Background(), "web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if !strings.Contains(gotPath, "/deployments/web") {
		t.Errorf("path = %s, want /deployments/web", gotPath)
	}
}

// ---------- getUserSettings ----------

func TestGetUserSettings(t *testing.T) {
	// The settings now live in a Secret. The API server returns "data"
	// (base64) and/or "stringData" (plain); the backend decodes "data" and
	// lets "stringData" override it.
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/secrets/user-settings") {
			t.Errorf("path = %s, want /secrets/user-settings", r.URL.Path)
		}
		resp := struct {
			StringData map[string]string `json:"stringData"`
			Data       map[string]string `json:"data"`
		}{
			// base64 of "ssh-ed25519 AAAA test@example.com" and "ghp_123"
			Data: map[string]string{
				"ssh-public-key": base64.StdEncoding.EncodeToString([]byte("ssh-ed25519 AAAA test@example.com")),
				"token.GITHUB":   base64.StdEncoding.EncodeToString([]byte("ghp_123")),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	data, err := c.getUserSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["ssh-public-key"] != "ssh-ed25519 AAAA test@example.com" {
		t.Errorf("ssh-public-key = %q", data["ssh-public-key"])
	}
	if data["token.GITHUB"] != "ghp_123" {
		t.Errorf("token.GITHUB = %q", data["token.GITHUB"])
	}
}

// TestGetUserSettings_StringDataPrecedence verifies a plain "stringData"
// entry overrides the decoded "data" for the same key.
func TestGetUserSettings_StringDataPrecedence(t *testing.T) {
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			StringData map[string]string `json:"stringData"`
			Data       map[string]string `json:"data"`
		}{
			StringData: map[string]string{"token.GITHUB": "from-stringdata"},
			Data:       map[string]string{"token.GITHUB": base64.StdEncoding.EncodeToString([]byte("from-data"))},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	data, err := c.getUserSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["token.GITHUB"] != "from-stringdata" {
		t.Errorf("token.GITHUB = %q, want stringData to override data", data["token.GITHUB"])
	}
}

func TestGetUserSettings_404(t *testing.T) {
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"secret not found"}`))
	}))
	defer server.Close()

	data, err := c.getUserSettings(context.Background())
	if err != nil {
		t.Fatalf("expected nil error for 404 (returns empty map), got %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty map, got %v", data)
	}
}

// ---------- upsertUserSettings ----------

// TestUpsertUserSettings_Create verifies the create path: POST -> 201, written
// with stringData (plain values), against the secrets collection.
func TestUpsertUserSettings_Create(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(201)
	}))
	defer server.Close()

	err := c.upsertUserSettings(context.Background(), map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %s, want POST (create)", gotMethod)
	}
	if !strings.Contains(gotPath, "/secrets") {
		t.Errorf("path = %s, want to hit /secrets", gotPath)
	}
	if !strings.Contains(gotBody, "stringData") {
		t.Errorf("body = %q, want stringData (plain values)", gotBody)
	}
	if !strings.Contains(gotBody, "value") {
		t.Errorf("body = %q, want contains value", gotBody)
	}
}

// TestUpsertUserSettings_Update verifies the update fallback: POST returns
// 409 (already exists) -> a PUT against the named Secret is issued.
func TestUpsertUserSettings_Update(t *testing.T) {
	var calls []string
	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.Method == "POST" {
			w.WriteHeader(409) // already exists
			return
		}
		if r.Method == "PUT" {
			if !strings.HasSuffix(r.URL.Path, "/secrets/user-settings") {
				t.Errorf("PUT path = %s, want /secrets/user-settings", r.URL.Path)
			}
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(400)
	}))
	defer server.Close()

	err := c.upsertUserSettings(context.Background(), map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (POST then PUT), got %v", calls)
	}
	if !strings.HasPrefix(calls[0], "POST ") {
		t.Errorf("first call = %q, want POST", calls[0])
	}
	if !strings.HasPrefix(calls[1], "PUT ") {
		t.Errorf("second call = %q, want PUT", calls[1])
	}
}

// ---------- handleDeployments (HTTP handler) ----------

func TestHandleDeployments(t *testing.T) {
	// A deployment with a valid workload-url annotation, and one without.
	// Only the annotated one should be returned by the handler.
	withURL := deployment{
		Metadata: struct {
			Name        string            `json:"name"`
			Namespace   string            `json:"namespace"`
			Annotations map[string]string `json:"annotations"`
		}{Name: "web", Namespace: "apps", Annotations: map[string]string{"workload-url": "/web/"}},
		Spec:   deploymentSpec{Replicas: 1},
		Status: deploymentStatus{ReadyReplicas: 1, Replicas: 1},
	}
	noURL := deployment{
		Metadata: struct {
			Name        string            `json:"name"`
			Namespace   string            `json:"namespace"`
			Annotations map[string]string `json:"annotations"`
		}{Name: "internal", Namespace: "apps"},
		Spec:   deploymentSpec{Replicas: 1},
		Status: deploymentStatus{ReadyReplicas: 1, Replicas: 1},
	}

	c, server := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This is the K8s API mock
		if strings.Contains(r.URL.Path, "/deployments") {
			resp := deploymentList{Items: []deployment{withURL, noURL}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	// Now test the HTTP handler
	req := httptest.NewRequest("GET", "/api/deployments", nil)
	rec := httptest.NewRecorder()
	c.handleDeployments(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "web") {
		t.Errorf("body = %q, want contains web (workload-url annotated)", body)
	}
	if strings.Contains(body, "internal") {
		t.Errorf("body = %q, want NOT contains internal (no workload-url annotation)", body)
	}
}

// ---------- handleHealthz ----------

func TestHandleHealthz(t *testing.T) {
	c, _ := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	c.handleHealthz(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("body = %q, want ok", body)
	}
}

// ---------- handleIndex ----------

// TestHandleIndex_ServesEmbeddedUI verifies the index handler serves the
// frontend that is compiled into the binary via //go:embed (web/index.html).
func TestHandleIndex_ServesEmbeddedUI(t *testing.T) {
	// Sanity: the embed must contain the real UI.
	embedded, err := webFS.ReadFile("web/index.html")
	if err != nil {
		t.Fatalf("embedded web/index.html not found in build: %v", err)
	}
	if !bytes.Contains(embedded, []byte("Devpod")) {
		t.Fatalf("embedded UI does not look like the Devpod dashboard (missing \"Devpod\")")
	}

	c, _ := newTestK8sClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	c.handleIndex(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if body := rec.Body.String(); body != string(embedded) {
		t.Errorf("served body does not match embedded web/index.html")
	}
}

// ---------- helper ----------

func TestJobStatusOf(t *testing.T) {
	// Condition-driven outcomes take priority.
	mkJob := func(conds []jobCondition, succeeded, failed, active int32) job {
		var j job
		j.Metadata.Name = "t"
		j.Status.Conditions = conds
		j.Status.Succeeded = succeeded
		j.Status.Failed = failed
		j.Status.Active = active
		return j
	}
	cases := []struct {
		name string
		j    job
		want string
	}{
		{"complete condition", mkJob([]jobCondition{{Type: "Complete", Status: "True"}}, 1, 0, 0), "succeeded"},
		{"failed condition", mkJob([]jobCondition{{Type: "Failed", Status: "True"}}, 0, 1, 0), "failed"},
		{"active -> running", mkJob(nil, 0, 0, 1), "running"},
		{"succeeded count only", mkJob(nil, 2, 0, 0), "succeeded"},
		{"failed count only", mkJob(nil, 0, 3, 0), "failed"},
		{"no signals -> pending", mkJob(nil, 0, 0, 0), "pending"},
	}
	for _, c := range cases {
		if got := jobStatusOf(c.j); got != c.want {
			t.Errorf("%s: jobStatusOf = %q, want %q", c.name, got, c.want)
		}
	}
}
