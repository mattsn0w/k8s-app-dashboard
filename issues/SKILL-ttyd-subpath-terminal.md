# Skill: ttyd Web Terminal in Kubernetes (Subpath-Aware)

## When to use
When deploying a web-based terminal (PTY over WebSocket) in a Kubernetes cluster
behind a gateway (Envoy Gateway, NGINX, Traefik, etc.), especially when each user
gets their own instance at a unique subpath on a shared domain.

## Why ttyd
- **Single static binary** (~1.3MB): no Node.js, no WASM, no native addons, no build
  toolchain. Runs on any Linux (glibc or musl).
- **Self-contained UI**: the xterm.js terminal is inlined in a single HTML file.
  No `/dist/`, no `/wasm/`, no external asset fetches.
- **Subpath-native via `-b` flag**: ttyd serves the page, WebSocket, and token
  endpoints under a configurable base path. No gateway URLRewrite needed.
- **Minimal image**: `ubuntu:24.04` + `bash` + the static ttyd binary ≈ 50MB.

## ttyd Key Flags (v1.7.8+)
| Flag | Meaning |
|------|---------|
| `-W` | Writable (allow client input). **Not** `-w` (that's `--cwd`). |
| `-p PORT` | Listen port (default 7681) |
| `-b BASE` | Base path for reverse-proxy subpath routing (e.g. `/u/user1/terminal`) |
| `-t key=value` | Client option (repeatable), e.g. `-t fontSize=14 -t theme=dark` |
| `-c user:pass` | Basic auth credential |
| `-H HEADER` | Auth header for proxy (lets gateway handle auth) |
| `-o` | Once: exit after one client disconnects |
| `-q` | Exit when all clients disconnect |
| `-i IFACE` | Bind interface (default `0.0.0.0`) |
| `-m N` | Max concurrent clients (0 = unlimited) |

## Subpath Routing Architecture
With `-b /u/user1/terminal`, ttyd serves:
- Page: `GET /u/user1/terminal/` → 200 (HTML)
- Redirect: `GET /u/user1/terminal` → 302 → `/u/user1/terminal/`
- WebSocket: `WS /u/user1/terminal/ws`
- Token: `GET /u/user1/terminal/token` → 200

The inlined page JS computes WS/token from `window.location.pathname`:
```js
const o = window.location.pathname.replace(/[/]+$/, "");
wsUrl = [proto, "//", host, o, "/ws", search].join("");
tokenUrl = [protocol, "//", host, o, "/token"].join("");
```
So the browser at `/u/user1/terminal/` opens WS at `/u/user1/terminal/ws` — matching ttyd's `-b` endpoints. **No URLRewrite filter needed.**

## Gateway Config (Envoy Gateway HTTPRoute)
```yaml
# Per-user terminal at /u/<user>/terminal
- backendRefs:
  - name: ttyd-user1
    port: 7681
  matches:
  - path:
      type: PathPrefix
      value: /u/user1/terminal
```
No URLRewrite. The `PathPrefix` match routes all subpath traffic to ttyd, which
serves natively at that base path.

## Dockerfile (ubuntu:24.04, multi-arch)
```dockerfile
FROM ubuntu:24.04
ARG TTYD_VERSION=1.7.8
ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates bash coreutils openssh-client less vim-tiny curl wget jq procps \
    && rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) ttyd_arch=x86_64 ;; \
      arm64) ttyd_arch=aarch64 ;; \
      *) echo "unsupported: ${TARGETARCH}"; exit 1 ;; \
    esac; \
    wget -qO /usr/local/bin/ttyd \
      "https://github.com/cloudtty/ttyd/releases/download/${TTYD_VERSION}/ttyd.${ttyd_arch}"; \
    chmod +x /usr/local/bin/ttyd; /usr/local/bin/ttyd --version
RUN useradd --create-home --shell /bin/bash terminal
USER terminal
WORKDIR /home/terminal
EXPOSE 7681
ENTRYPOINT ["/usr/local/bin/ttyd"]
CMD ["-W", "-p", "7681", "bash"]
```

## Kubernetes Deployment (per-user instance)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: terminal-user1
  namespace: apps
spec:
  replicas: 1
  selector:
    matchLabels: { app: terminal-user1 }
  template:
    metadata:
      labels: { app: terminal-user1 }
    spec:
      containers:
      - name: ttyd
        image: registry.co.slakin.net/local/ttyd-web:latest
        args: ["-W", "-p", "7681", "-b", "/u/user1/terminal", "bash"]
        ports:
        - containerPort: 7681
        readinessProbe:
          httpGet:
            path: /u/user1/terminal/token
            port: 7681
          initialDelaySeconds: 2
        resources:
          requests: { cpu: 10m, memory: 32Mi }
          limits: { cpu: 200m, memory: 128Mi }
```

## Testing Protocol (≤5 min per test)
- **Always use `timeout 30` (or less) on blocking commands** (docker run, node ws,
  curl with long connections). Never let a test hang.
- **Verify in this order** (fast → slow):
  1. `docker run --rm ttyd-web:local ttyd --version` (1s)
  2. Container up + `curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:PORT/base/token` (3s)
  3. WS open test via `node -e` with a 4s timeout (5s)
  4. Browser verification via Chrome extension (optional, for visual/interactive confirmation)
- **If a test hangs >5s**, kill it, check `docker logs`, fix the arg/config, retry once.

## Gotchas
- **`-W` not `-w`**: `-w` is `--cwd` (working directory). `-W` is `--writable`.
- **ttyd only serves at its base path**: with `-b /foo`, a request to `/` returns 404.
- **No URLRewrite needed**: ttyd's `-b` handles subpath natively. Adding a gateway
  URLRewrite on top would double-rewrite and break the WS path.
- **Read-only by default**: without `-W`, the terminal is view-only (no input).
- **PTY permissions**: the container user needs access to `/dev/ptmx` and `/dev/pts`.
  The default `ubuntu:24.04` setup works out of the box for a single-user terminal.
- **Per-user isolation**: each user gets their own pod/container with their own `-b`
  value. One ttyd instance serves one base path — it cannot multiplex multiple users.

## cloudtty (operator) vs. standalone ttyd
| | cloudtty operator | standalone ttyd |
|---|---|---|
| Architecture | K8s operator + CRD + auto-created pods | Plain Deployment |
| Base image | Alpine (musl), root | Ubuntu 24.04 (glibc), non-root |
| Subpath | NodePort/Ingress (no subpath concept) | `-b` flag (native subpath) |
| Extensibility | Fixed image (kubectl/helm/yq) | Add any packages |
| K8s coupling | Requires operator | None |
| Use case | K8s cloud shell (kubectl in browser) | Generic web terminal per user |

For per-user terminals at unique subpaths, **standalone ttyd** is the right fit.
