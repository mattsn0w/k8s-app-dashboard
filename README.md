# Devpod

A single-page Kubernetes deployment control panel. Manage deployments, view logs, and inspect cluster state from one lightweight web UI — no heavy frontend framework, no client-go, no external dependencies.

```
┌──────────────────────────────────────────────────────────┐
│  Devpod  Control Panel                    ☀️  ⚙️       │
├──────────────────────────────────────────────────────────┤
│  Deployments                                            │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Name        Status              Actions          │  │
│  │  echo-server  ● 2 containers      📋 ⚡ 🗑  [ON] │  │
│  │  nginx        ● 1 container       📋 ⚡ 🗑  [ON] │  │
│  │  hello-world  ○ 0 containers      📋 ⚡ 🗑  [OFF]│  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

## Features

- **Deployment table** — live view of all deployments in the target namespace with container count, replica status, and running state
- **Toggle on/off** — scale a deployment to 0 or restore it to its previous replica count with one click (the last count is saved in an annotation)
- **Real-time logs** — streaming log viewer using WebSocket with auto-reconnect, backed by the K8s `pods/log?follow=true` API
- **Terminate pods** — delete all pods for a deployment (the Deployment controller recreates them)
- **Delete deployment** — permanently remove a deployment
- **Workload URLs** — deployments annotated with `workload-url` get an external link in the table
- **User settings** — store SSH public keys and access tokens in a Secret, accessible from the UI
- **View modes** — **Basic** (default; only the Deployments tab) and **Advanced** (also shows the Jobs and Cluster nodes tabs). Persisted in the `user-settings` Secret via a **Modes** section in the user menu.
- **Theme switching** — Catppuccin Latte (light) and Frappe (dark) themes, persisted in the `user-settings` Secret
- **Subpath support** — works when served under any path (e.g. `/apps/dashboard/`) thanks to relative API calls and dynamic WebSocket URL construction

## Architecture

```
Browser ──HTTPS──▶ Envoy Gateway ──▶ deploy-dashboard Pod
                   (port 80/443)     (port 8080)
                                        │
                                        ▼
                                   K8s API Server
                                   (in-cluster,
                                    service account token)
```

The backend is a single Go binary (~7 MB, statically linked) that talks to the Kubernetes API server directly using the in-cluster service account token. It uses **only the Go standard library** — no `client-go`, no `k8s.io/*` packages. This means it can be built in restricted environments where module proxy access is unavailable.

### HTTP API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Serve the SPA (embedded or from `web/index.html`) |
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/api/deployments` | List deployments with pod status |
| `GET` | `/api/pods?deployment=name` | List pods for a deployment |
| `POST` | `/api/scale?deployment=name` | Scale a deployment (`{"replicas": N}`) |
| `POST` | `/api/kill-pods?deployment=name` | Delete all pods for a deployment |
| `DELETE` | `/api/delete-deployment?deployment=name` | Delete a deployment |
| `GET` | `/api/logs?pod=name&container=name` | Stream pod logs (HTTP, `follow=true`) |
| `GET` | `/api/logs-ws` | WebSocket log stream (upgrade from HTTP) |
| `GET` | `/api/nodes` | List cluster nodes |
| `GET/PUT` | `/api/user-settings` | Read/write user settings ConfigMap |
| `GET/PUT` | `/api/tokens` | Read/write access tokens |

### WebSocket Log Protocol

The `/api/logs-ws` endpoint upgrades to WebSocket (RFC 6455) on the same HTTP port. The client sends JSON commands:

```json
{"action": "follow", "pod": "web-abc123", "container": "web"}
{"action": "stop"}
```

The server responds with text frames containing log lines. The implementation includes a 1 MB frame size cap and a writer goroutine to serialize concurrent writes.

## Building

### Prerequisites

- Go 1.22+ (tested with 1.27)
- No external dependencies (stdlib only)

### Static Binary

```bash
CGO_ENABLED=0 go build -ldflags="-s -w -extldflags=-static" -o deploy-dashboard ./cmd/
```

The resulting binary is ~7 MB, statically linked, and runs on any Linux system (Alpine, Debian, etc.) without glibc or musl.

### Docker

```bash
docker build -t registry.local/deploy-dashboard:latest .
docker push registry.local/deploy-dashboard:latest
```

The `Dockerfile` uses a two-stage build: `golang:1.27-alpine` for compilation, `alpine:3.20` + `ca-certificates` for the runtime.

### Embedding the Frontend

The SPA is embedded into the binary at build time via `embed_gen.go`, which base64-encodes `web/index.html`. To regenerate:

```bash
# Generate the base64-encoded HTML
base64 -w0 web/index.html > /tmp/html.b64

# Update embed_gen.go (or use the EMBEDDED_HTML env var at runtime)
```

At runtime, the `EMBEDDED_HTML` environment variable (base64) overrides the compiled-in HTML. This allows updating the UI without rebuilding the binary.

## Running

### In a Kubernetes Cluster

```bash
# 1. Create the namespace
kubectl apply -f manifests/01-namespace.yaml

# 2. Create RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding)
kubectl apply -f manifests/02-rbac.yaml

# 3. Deploy the dashboard
kubectl apply -f manifests/03-deployment.yaml

# 4. (Optional) Deploy demo workloads
kubectl apply -f manifests/04-demo-workloads.yaml
```

The manifest in `03-deployment.yaml` uses a `debian:bookworm-slim` pod with the binary injected via `kubectl cp` or `tar` pipe. For production, replace with a standard `Deployment` using the pushed image.

### Locally

```bash
# Point at a kubeconfig (outside the cluster)
./deploy-dashboard --kubeconfig=~/.kube/config --namespace=default --listen=:8080

# Or with env vars for in-cluster-style auth
KUBERNETES_SERVICE_HOST=1.2.3.4 KUBERNETES_SERVICE_PORT=6443 \
  KUBERNETES_INSECURE=1 \
  ./deploy-dashboard --namespace=default
```

| Flag | Default | Description |
|------|---------|-------------|
| `--kubeconfig` | *(empty = in-cluster)* | Path to a kubeconfig file |
| `--namespace` | `default` | Namespace to watch |
| `--listen` | `:8080` | Listen address |

## RBAC

The `ServiceAccount` `deploy-dashboard` is granted:

| Resource | Verbs |
|----------|-------|
| `pods`, `nodes`, `services` | `get`, `list`, `watch` |
| `pods/log` | `get` |
| `deployments`, `replicasets` | `get`, `list`, `watch`, `update`, `patch` |
| `deployments` | `create`, `delete` |
| `configmaps` (namespace-scoped) | `get`, `list`, `watch`, `create`, `update`, `patch` |

## Frontend

The SPA is a single `index.html` file (~500 lines) that loads [Primer CSS](https://primer.style/) from a CDN and uses vanilla JavaScript. Key design decisions:

- **Relative API paths** — all `fetch()` calls use relative paths (`api/deployments`, not `/api/deployments`) so the app works under any subpath
- **Dynamic WebSocket URL** — constructed from `location.pathname` + `location.host` to support subpath deployment
- **Catppuccin → Primer mapping** — Latte maps to `data-color-mode="light"`, Frappe maps to `data-color-mode="dark"`, with CSS custom properties bridging the two palettes
- **Octicons** — inline SVG icons (gear, key, lock, trash, pulse, sun, moon, etc.)
- **Auto-reconnect** — the WebSocket log viewer reconnects after 3 seconds on close or error

## Testing

```bash
# Go unit tests (53 tests: WebSocket frame codec, URL validation, K8s client)
make test

# JavaScript unit tests (28 tests: theme, API paths, log parsing, deployment state)
make test-js

# Static analysis
make vet

# Build
make build
```

The CI workflow (`.github/workflows/ci.yml`) runs `go vet`, `go test -race`, static build, and `node web/test.js` on every push and PR.

See [TEST_PLAN.md](TEST_PLAN.md) for the full test strategy.

## Project Structure

```
deploy-dashboard/
├── cmd/
│   ├── main.go          # Backend: K8s client, HTTP handlers, WebSocket
│   ├── ws_test.go       # WebSocket frame codec tests (36 tests)
│   ├── url_test.go      # URL validation tests (16 tests)
│   └── main_test.go     # K8s client + HTTP handler tests (14 tests)
├── web/
│   ├── index.html       # Frontend SPA (Primer CSS + vanilla JS)
│   └── test.js          # Frontend unit tests (28 tests)
├── manifests/
│   ├── 01-namespace.yaml
│   ├── 02-rbac.yaml
│   ├── 03-deployment.yaml
│   └── 04-demo-workloads.yaml
├── embed_gen.go         # Base64-embedded HTML for the binary
├── Dockerfile           # Two-stage build (golang → alpine)
├── Makefile             # test, test-js, vet, build, fmt
├── go.mod               # module deploy-dashboard (no dependencies)
├── TEST_PLAN.md
└── .github/workflows/ci.yml
```
