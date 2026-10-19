# Devpod — Code Review: Anti-Patterns, Security, UX

> Scope: `cmd/main.go`, `web/index.html`, `manifests/`, `Dockerfile`, `build-and-deploy.sh`
> Date: 2025-07-19

---

## 1. Security Vulnerabilities

### 1.1 No Authentication or Authorization on the Dashboard HTTP API
**Severity: HIGH**

The dashboard exposes `POST /api/scale`, `POST /api/kill-pods`, `DELETE /api/delete-deployment`, `PUT /api/user-settings`, and `PUT /api/tokens` to **anyone** who can reach the Service. There is no authentication — no OAuth, no OIDC, no API key, not even a shared secret. In the current deployment, the Envoy Gateway routes `apps01.co.slakin.net` → the dashboard with no authn/authz filters.

An attacker who can reach the LoadBalancer IP (172.16.1.90) or the NodePort can:
- Delete any deployment in the `apps` namespace
- Scale deployments to 0 (DoS)
- Read all pod logs (information disclosure)
- Read/write the `user-settings` ConfigMap (exfiltrate stored SSH keys and access tokens)

**Fix:** Add at minimum one of:
- OIDC authentication via the Gateway (e.g., Envoy Gateway's `JWT` or `OAuth2` auth policy)
- A shared-secret header checked in a Go middleware
- A network policy restricting access to a specific CIDR (VPN, office network)
- As a stopgap, move the dashboard to a `NodePort` behind a reverse proxy with basic auth

### 1.2 Access Tokens and SSH Keys Stored in Plaintext in a ConfigMap
**Severity: HIGH**

The `user-settings` ConfigMap stores `ssh-public-key` and `token.*` entries in **plaintext** (no `binaryData`). ConfigMaps are:
- Base64-encoded (not encrypted) in etcd
- Readable by any pod in the namespace with `get` on ConfigMaps
- Visible via `kubectl get cm -o yaml` to anyone with cluster read access
- Included in `etcd` snapshots

**Fix:** Use a `Secret` instead of a ConfigMap for sensitive data. Or, if a ConfigMap is acceptable, document the risk and add a `Secret` for tokens. At minimum, the `GET /api/user-settings` and `GET /api/tokens` endpoints should mask token values (e.g., return `ghp_1234…` instead of the full token).

### 1.3 K8s API Errors Leaked to the Client
**Severity: MEDIUM**

Every handler calls `http.Error(w, err.Error(), ...)` which dumps the raw K8s API error to the browser. This includes:
- The API server URL (`https://10.43.0.1:443/...`)
- The full JSON error body from the API server
- RBAC denial messages that reveal the ServiceAccount name and the exact resource/verb that was denied

Example: `GET /api/v1/namespaces/apps/deployments/web: 403 Forbidden: deployments.apps "web" is forbidden: User "system:serviceaccount:apps:deploy-dashboard" cannot get resource "deployments" in API group "apps" in namespace "apps"`

**Fix:** Log the full error server-side (`log.Printf`) and return a sanitized message to the client (`"internal error"`, `"forbidden"`, `"not found"`).

### 1.4 `KUBERNETES_INSECURE=1` Disables TLS Verification
**Severity: MEDIUM**

The `newK8sClient` function supports `KUBERNETES_INSECURE=1` which sets `InsecureSkipVerify: true` on the TLS config. If this env var is accidentally set in production, all traffic to the K8s API is unencrypted and unverified — vulnerable to MitM.

**Fix:** Remove the env var support entirely, or gate it behind a build tag / flag that is clearly documented as dev-only. Log a prominent warning if it's enabled.

### 1.5 No Rate Limiting
**Severity: LOW-MEDIUM**

There is no rate limiting on any endpoint. A single client can:
- Rapidly call `POST /api/scale` in a loop, hammering the K8s API
- Open hundreds of concurrent WebSocket log streams
- Call `DELETE /api/delete-deployment` repeatedly

**Fix:** Add a simple per-connection rate limiter (e.g., `golang.org/x/time/rate` or a token bucket). At minimum, cap concurrent WebSocket connections.

### 1.6 WebSocket Has No Origin Validation
**Severity: LOW-MEDIUM**

The WebSocket upgrade in `handleLogsWS` does not validate the `Origin` header. Any website the user visits could open a WebSocket to the dashboard and stream logs (if the dashboard is exposed over HTTPS with a valid cert, or if the user has an open port-forward).

**Fix:** Check the `Origin` header against an allowlist of expected origins.

### 1.7 No Request Size Limit on JSON Bodies
**Severity: LOW**

`PUT /api/user-settings` and `PUT /api/tokens` decode `r.Body` without a size limit. A client can send a multi-GB JSON body, causing the pod to OOM.

**Fix:** Wrap `r.Body` in `http.MaxBytesReader(w, r.Body, 1<<20)` (1 MB is more than enough).

### 1.8 `workload-url` Annotation Allows `javascript:` URIs (Partial)
**Severity: LOW**

The `isValidWorkloadURL` function rejects `javascript:`, `data:`, `file:`, etc. by only allowing `http://`, `https://`, or `/` prefixes. However, the annotation is set by **anyone** who can edit the Deployment (which is the same as anyone who can reach the dashboard API — see 1.1). A `javascript:` URL in an annotation would be rejected, but the real risk is that the frontend renders `href="${d.workloadUrl}"` directly in innerHTML.

**Fix:** In the frontend, validate that `workloadUrl` starts with `/` or `https://` before rendering. Use `textContent` instead of `innerHTML` for the URL value.

---

## 2. Anti-Patterns / Bad Practices

### 2.1 `embed_gen.go` is a 30 KB Base64 Blob in Source Control
**Severity: HIGH (maintainability)**

`embed_gen.go` contains a ~30 KB base64-encoded copy of `web/index.html`. This means:
- Every HTML change requires regenerating this file and committing a 30 KB diff that is unreadable
- The `embed_gen.go` file is **stale** — it contains the old "Deploy Dashboard" HTML with Catppuccin Mocha/Frappe/Latte themes and emoji icons, while `web/index.html` has the new Primer CSS version with Octicons. The running app uses the `EMBEDDED_HTML` env var (from the ConfigMap), so the embedded HTML is dead code, but it's confusing.
- `Dockerfile` does `go build ./cmd/` which would embed the **stale** HTML, not the current one.

**Fix:** Use Go's `embed` package:
```go
//go:embed web/index.html
var embeddedHTML string
```
This requires `web/index.html` to be in the same module. Alternatively, remove `embed_gen.go` entirely and rely on the `EMBEDDED_HTML` env var or the disk fallback.

### 2.2 Naive Kubeconfig Parser
**Severity: MEDIUM**

The `newK8sClient` function parses kubeconfig by looking for lines starting with `server:` and `token:`. This:
- Doesn't handle multi-cluster kubeconfigs (it picks the last `server:` line)
- Doesn't handle `user` vs `cluster` separation in kubeconfig
- Doesn't handle `client-certificate-data` or `client-key-data` auth
- Doesn't handle `insecure-skip-tls-verify: true` in the kubeconfig
- Doesn't handle base64-encoded certificate data

**Fix:** Either use a proper YAML parser (the `gopkg.in/yaml.v3` package is a single dependency) or document clearly that only single-cluster kubeconfigs with bearer token auth are supported.

### 2.3 Pod Instead of Deployment
**Severity: MEDIUM**

`manifests/03-deployment.yaml` defines a raw `Pod` named `dashboard-runner` instead of a `Deployment`. This means:
- No automatic restart on node failure (only `restartPolicy: Always` within the node)
- No rolling updates
- No replica management
- No HPA
- The pod spec is 1:1 with a single node — if the node dies, the app is down

**Fix:** Convert to a `Deployment` with at least 1 replica. The `Dockerfile` already produces a proper image; use it.

### 2.4 No Readiness or Liveness Probes
**Severity: MEDIUM**

The pod in `03-deployment.yaml` has no `readinessProbe` or `livenessProbe`. The binary has a `/healthz` endpoint that is never wired up. This means:
- K8s considers the pod ready immediately after the container starts, before the Go binary has bound its port
- A hung binary is never restarted

**Fix:** Add:
```yaml
readinessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 2
  periodSeconds: 5
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

### 2.5 `build-and-deploy.sh` Uses `kubectl cp`
**Severity: MEDIUM**

The script uses `kubectl cp` to copy the binary into the pod. As discovered during development, `kubectl cp` frequently fails with "broken pipe" for files >1 MB. The `tar` pipe workaround (`tar cf - | kubectl exec -i ... tar xf -`) is more reliable but is not used here.

**Fix:** Replace `kubectl cp` with:
```bash
tar cf - /tmp/deploy-dashboard-static | kubectl exec -i -n "$NS" "$POD_NAME" -- tar xf - -C /app/
```
Or better, build a Docker image and use a Deployment (see 2.3).

### 2.6 RBAC Missing ConfigMap Permissions
**Severity: MEDIUM**

The `ClusterRole` in `02-rbac.yaml` does **not** grant `get`, `list`, `watch`, `create`, `update`, or `patch` on `configmaps`. However, the app uses a `user-settings` ConfigMap for storing SSH keys and tokens. This means the RBAC manifest is incomplete — the app will get 403 errors on `/api/user-settings` unless a separate namespace-scoped Role was added manually (as was done during development).

**Fix:** Add to the ClusterRole (or a namespace-scoped Role):
```yaml
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
```

### 2.7 RBAC Grants Cluster-Wide Read on Nodes
**Severity: LOW**

The ClusterRole grants `get`, `list`, `watch` on `nodes` cluster-wide. The `/api/nodes` endpoint returns node names, roles, and kubelet versions. This is more than a deployment dashboard needs. If the dashboard is compromised (see 1.1), an attacker gets a map of the cluster.

**Fix:** Consider whether the node listing is essential. If so, restrict to the specific nodes running the `apps` namespace (via node selector labels). If not, remove the permission and the `/api/nodes` endpoint.

### 2.8 `deletePods` Does N Sequential API Calls
**Severity: LOW**

`deletePods` calls `DELETE /pods/<name>` in a loop for each pod. For a deployment with 10 pods, this is 10 sequential HTTP roundtrips. The K8s API supports batch delete via `DELETE /pods?labelSelector=...`.

**Fix:** Use a single `DELETE` with the label selector:
```go
path := "/api/v1/namespaces/" + *namespace + "/pods?labelSelector=" + url.QueryEscape(sel)
c.do(ctx, "DELETE", path, nil, nil)
```

### 2.9 `wsReadFrame` and `wsReadFrameReader` are Duplicated
**Severity: LOW (code quality)**

`wsReadFrame(rw *bufio.ReadWriter)` and `wsReadFrameReader(r io.Reader)` have identical logic — the only difference is the reader type. Since `*bufio.ReadWriter` implements `io.Reader`, `wsReadFrame` is dead code (only `wsReadFrameReader` is used in production).

**Fix:** Delete `wsReadFrame`. Keep `wsReadFrameReader`.

### 2.10 `handleIndex` Has a Disk Fallback That Reads on Every Request
**Severity: LOW**

When `embeddedHTML` is empty (which is the case if `EMBEDDED_HTML` env var is not set), `handleIndex` calls `os.ReadFile("web/index.html")` on **every request**. This is a disk I/O call in the request path.

**Fix:** Read the file once at startup and cache it. Or, make the embedded HTML mandatory (fail at startup if neither `EMBEDDED_HTML` nor the file exists).

### 2.11 No Graceful Shutdown
**Severity: LOW**

`main()` calls `srv.ListenAndServe()` which blocks forever. There is no signal handling — `SIGTERM` (sent by K8s on pod termination) kills the process immediately, dropping all active WebSocket connections and in-flight HTTP requests.

**Fix:** Use `signal.NotifyContext` and `srv.Shutdown(ctx)` with a timeout:
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
// ...
go func() {
    <-ctx.Done()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    srv.Shutdown(shutdownCtx)
}()
```

### 2.12 `go.mod` Says `go 1.27.0` but No `go.sum`
**Severity: LOW**

`go.mod` declares `go 1.27.0` but there is no `go.sum` file. The `Dockerfile` does `COPY go.mod go.sum ./` which will fail because `go.sum` doesn't exist. Since there are no external dependencies, this works in practice (Go skips the download), but the Dockerfile is technically broken.

**Fix:** Either remove the `COPY go.sum` line from the Dockerfile, or generate a (empty) `go.sum`.

### 2.13 ConfigMap HTML is Out of Sync with `web/index.html`
**Severity: MEDIUM**

The `dashboard-html` ConfigMap in `03-deployment.yaml` contains a base64-encoded copy of an **older** version of `index.html` (pre-Primer, with emoji icons and the "Deploy Dashboard" title). The current `web/index.html` has Primer CSS, Octicons, and the "Devpod" title. The running app uses the ConfigMap, so users see the old UI.

**Fix:** Regenerate the ConfigMap: `base64 -w0 web/index.html` and update the manifest. Better: use the `EMBEDDED_HTML` env var from a build step, or use Go's `embed` package (see 2.1).

### 2.14 `build-and-deploy.sh` Hardcodes NodePort 31143
**Severity: LOW**

The script's final message says "Access via NodePort: http://<node-ip>:31143/" but the Service in `03-deployment.yaml` is `type: ClusterIP` (no NodePort). The 31143 reference is stale.

**Fix:** Remove the NodePort reference or change the Service to `NodePort` if intended.

---

## 3. User Experience Improvements

### 3.1 `alert()` for All Error Messages
**Severity: HIGH (UX)**

Every error in the frontend uses `alert()`:
- "Failed to scale web: ..."
- "Failed to terminate pods: ..."
- "Failed to delete deployment: ..."
- "Failed to fetch pods: ..."
- "No pods found for web"
- "Failed to save: ..."

`alert()` blocks the JavaScript event loop, is impossible to style, and is jarring. It also prevents the user from seeing the error context (which deployment, which action) because the dialog covers the screen.

**Fix:** Use an inline toast/snackbar notification system. A simple implementation:
```javascript
function showToast(msg, type = 'error') {
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.textContent = msg;
  document.body.appendChild(toast);
  setTimeout(() => toast.classList.add('show'), 10);
  setTimeout(() => { toast.classList.remove('show'); setTimeout(() => toast.remove(), 300); }, 5000);
}
```

### 3.2 `confirm()` for Destructive Actions
**Severity: MEDIUM (UX)**

`confirm()` is used for "Terminate pods" and "Delete deployment". This is:
- Ugly (native browser dialog)
- Impossible to style
- Blocks the event loop
- Doesn't let the user see the deployment details while confirming

**Fix:** Use a custom confirmation modal (the app already has modal infrastructure). Show the deployment name, current replica count, and a "Type the name to confirm" pattern for delete (like GitHub does).

### 3.3 No Loading State on Action Buttons
**Severity: MEDIUM (UX)**

When the user clicks "Stop", "Start", "Terminate", or "Delete", the button doesn't change. The user sees no feedback until the 1–1.5 second `setTimeout(fetchDeployments)` fires and the table re-renders. During this window, the user might click the button again, causing a double API call.

**Fix:** Disable the button and show a spinner (or change the icon to a loading state) while the API call is in flight. Re-enable after the response.

### 3.4 No Optimistic Update for Toggle
**Severity: LOW (UX)**

The toggle switches to the new state immediately (good), but if the API call fails, the UI reverts silently after the `alert()`. The user has to look carefully to notice the toggle went back.

**Fix:** On failure, show a toast (see 3.1) and add a brief "error" animation to the toggle (e.g., shake or flash red).

### 3.5 Log Modal: No Pod/Container Selector
**Severity: MEDIUM (UX)**

`showLogs()` automatically picks the first ready pod (or first pod) and the first container. If a deployment has multiple pods or containers, the user cannot choose which one to view. There is no dropdown or tab in the log modal.

**Fix:** Add a pod selector (dropdown) and container selector (dropdown) to the log modal header. Populate them from `api/pods?deployment=...`. Allow the user to switch pods/containers without closing and reopening the modal.

### 3.6 Log Modal: No Pause/Resume
**Severity: LOW (UX)**

The log stream is always "follow" mode. There is no way to pause the stream while reading. If the pod is logging rapidly (e.g., a debug loop), the user cannot read the output because it scrolls too fast.

**Fix:** Add a pause/resume button in the log modal footer. When paused, stop appending new lines but keep the WebSocket open. When resumed, catch up.

### 3.7 Log Modal: No Copy or Download
**Severity: LOW (UX)**

There is no way to copy the log output to the clipboard or download it as a file. For sharing with colleagues or filing bugs, the user has to manually select and copy text from the modal.

**Fix:** Add "Copy" and "Download" buttons to the log modal footer.

### 3.8 Log Modal: No Log Level Filtering
**Severity: LOW (UX)**

All log lines are displayed with equal styling. There is no color-coding by log level (INFO, WARN, ERROR) or filtering.

**Fix:** Parse the log line format (if consistent) and apply CSS classes for ERROR (red), WARN (yellow), INFO (default). Add a filter dropdown (All / Errors / Warnings).

### 3.9 No Namespace Indicator
**Severity: LOW (UX)**

The header says "Devpod Control Panel" but doesn't show which namespace the user is viewing. If the dashboard is deployed with `--namespace=apps`, the user has no visual indication. If they have access to multiple namespaces (via different deployments of the dashboard), they can't tell which one they're looking at.

**Fix:** Add the namespace to the subtitle: "Devpod · Control Panel · `apps`". Fetch it from a new `/api/info` endpoint or from the deployment list response.

### 3.10 No Keyboard Shortcuts
**Severity: LOW (UX)**

There are no keyboard shortcuts. Power users have to click every button.

**Fix:** Add:
- `L` — open logs for the selected/hovered deployment
- `S` — toggle the selected deployment
- `Esc` — close modal (already works for the log modal)
- `?` — show keyboard shortcut help

### 3.11 10-Second Polling is Too Slow for a Control Panel
**Severity: MEDIUM (UX)**

The frontend polls `api/deployments` every 10 seconds. This means:
- After a scale operation, the user waits up to 10 seconds to see the real state (the 1.5 second `setTimeout` helps, but it's a band-aid)
- If a pod crashes and the controller restarts it, the user sees the crash for up to 10 seconds
- The "N containers running" badge can be stale

**Fix:** Reduce polling to 3–5 seconds. Or, better, use a WebSocket/SSE stream for deployment status changes (the app already has WebSocket infrastructure). Or, trigger an immediate refresh after any mutation (scale, kill, delete) — the 1.5 second `setTimeout` already does this, but it should also update the status badge from the response.

### 3.12 No "Last Updated" Timestamp
**Severity: LOW (UX)**

The user has no indication of when the data was last fetched. If the polling silently fails (network issue), the table shows stale data with no warning.

**Fix:** Add a "Last updated: 12:34:56" timestamp in the section title or a status bar. If the fetch fails, show "Last updated: 12:34:56 (stale)".

### 3.13 Help/Docs Link Points to `http://example.co.slakin.net`
**Severity: LOW (UX)**

The "Help/Docs" menu item links to `http://example.co.slakin.net` which is a placeholder. This is a dead end for users.

**Fix:** Point to the actual documentation URL, or remove the link until real docs exist.

### 3.14 SSH Key Modal: No Key Validation
**Severity: LOW (UX)**

The SSH key modal accepts any text. The user can paste garbage and save it. There's no validation that the key looks like a valid SSH public key.

**Fix:** Validate that the key matches the pattern `^ssh-(ed25519|rsa|ecdsa)[ -]` and has a comment. Show an inline error if the key is invalid.

### 3.15 Token Modal: Fixed 3 Rows
**Severity: LOW (UX)**

The token modal has exactly 3 rows for key/value pairs. If the user wants to store 4 or more tokens, they can't. If they only need 1, the 2 empty rows are visual noise.

**Fix:** Make the rows dynamic: start with 1 row, add an "Add token" button to add more, add a remove button (×) on each row.

### 3.16 No Empty State Illustration
**Severity: LOW (UX)**

When there are no deployments, the table shows a cloud icon and "No deployments found in this namespace." This is functional but bland.

**Fix:** Add a more illustrative empty state with a suggestion ("Try creating a deployment" or "Check that you're in the right namespace").

### 3.17 Table: No Sorting
**Severity: LOW (UX)**

The deployment table has no column sorting. With many deployments, finding a specific one requires scanning.

**Fix:** Add click-to-sort on the Name and Status columns. Default sort by name.

### 3.18 Table: No Search/Filter
**Severity: LOW (UX)**

There is no search box to filter deployments by name. With 20+ deployments, the table becomes unwieldy.

**Fix:** Add a search input above the table that filters rows by name (client-side, no API call needed).

---

## 4. Priority Matrix

| # | Item | Category | Severity | Effort |
|---|------|----------|----------|--------|
| 1.1 | No auth on API | Security | HIGH | M |
| 1.2 | Tokens in plaintext ConfigMap | Security | HIGH | S |
| 2.1 | Stale embed_gen.go / 30 KB blob | Anti-pattern | HIGH | S |
| 3.1 | alert() for errors | UX | HIGH | S |
| 2.3 | Pod instead of Deployment | Anti-pattern | M | S |
| 2.4 | No probes | Anti-pattern | M | S |
| 2.6 | RBAC missing configmap perms | Anti-pattern | M | S |
| 2.13 | ConfigMap HTML out of sync | Anti-pattern | M | S |
| 1.3 | K8s errors leaked | Security | M | S |
| 1.4 | KUBERNETES_INSECURE | Security | M | S |
| 2.2 | Naive kubeconfig parser | Anti-pattern | M | M |
| 2.5 | kubectl cp in build script | Anti-pattern | M | S |
| 3.2 | confirm() for destructive actions | UX | M | M |
| 3.3 | No loading state on buttons | UX | M | S |
| 3.5 | No pod/container selector in logs | UX | M | M |
| 3.11 | 10s polling too slow | UX | M | S |
| 1.5 | No rate limiting | Security | L-M | S |
| 1.6 | No WS origin validation | Security | L-M | S |
| 1.7 | No request size limit | Security | L | S |
| 1.8 | workload-url injection | Security | L | S |
| 2.7 | Cluster-wide node read | Anti-pattern | L | S |
| 2.8 | Sequential pod deletes | Anti-pattern | L | S |
| 2.9 | Duplicated wsReadFrame | Anti-pattern | L | S |
| 2.10 | Disk read on every request | Anti-pattern | L | S |
| 2.11 | No graceful shutdown | Anti-pattern | L | S |
| 2.12 | go.sum missing | Anti-pattern | L | S |
| 2.14 | Stale NodePort reference | Anti-pattern | L | S |
| 3.4 | No optimistic update feedback | UX | L | S |
| 3.6 | No log pause/resume | UX | L | M |
| 3.7 | No log copy/download | UX | L | S |
| 3.8 | No log level filtering | UX | L | M |
| 3.9 | No namespace indicator | UX | L | S |
| 3.10 | No keyboard shortcuts | UX | L | M |
| 3.12 | No last-updated timestamp | UX | L | S |
| 3.13 | Placeholder Help/Docs URL | UX | L | S |
| 3.14 | No SSH key validation | UX | L | S |
| 3.15 | Fixed 3 token rows | UX | L | S |
| 3.16 | Bland empty state | UX | L | S |
| 3.17 | No table sorting | UX | L | S |
| 3.18 | No search/filter | UX | L | S |

**Legend:** Effort: S = < 1 hour, M = 1–4 hours, L = > 4 hours
