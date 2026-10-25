Replace the 'hello-world' demo app in `manifests/04-demo-workloads.yaml` with the ghostty-web app as a container.

Sources and documentation for ghostty-web application are located in the directory `/home/msnow/Projects/kubernetes/github.com/coder__ghostty-web`.

Build a Dockerfile for this application to run inside an Ubuntu 24.04 or 26.04 image.

Push the image to `registry.co.slakin.net/local/ghosttyweb:latest` which is a local to the network registry and does not require auth.

---
- **Status:** IN PROGRESS

## Implementation notes
- **Image build dir:** `k8s-app-dashboard/ghosttyweb-image/`
  - `Dockerfile` — multi-stage on `ubuntu:24.04`: stage 1 installs Node + `npm ci`
    (pinned deps in `package.json`/`package-lock.json`) so `@lydell/node-pty` pulls the
    prebuilt `linux-x64` binary; stage 2 is minimal ubuntu:24.04 + Node + `libstdc++6`,
    copies `node_modules` + `bin/`, runs `node bin/demo.js` as a non-root `appuser`.
    Default env: `PORT=8080`, `HOST=0.0.0.0`, `GHOSTTY_ALLOWED_HOSTS=apps01.co.slakin.net`.
  - `build-and-push.sh` — `docker build -t registry.co.slakin.net/local/ghosttyweb:latest` + push (auth-free).
  - App build validated in the sandbox (deps install, server boots, PTY works).
  - **Note:** "26.04" does not exist yet → using `ubuntu:24.04`.

- **Manifests:**
  - `04-demo-workloads.yaml` — `hello-world` (multi-container busybox) replaced by a
    single-container **`ghostty-web`** Deployment + Service (image
    `registry.co.slakin.net/local/ghosttyweb:latest`, port 8080, env above, probes,
    resource limits). `workload-url: /apps/terminal`.
  - `03-deployment.yaml` — HTTPRoute rule: browser `PathPrefix /apps/terminal` →
    `ghostty-web:8080`, with a `URLRewrite` filter (`path.type: ReplaceFullPath`,
    `replaceFullPath: /`). Server dry-run validates.

## Routing finding (BLOCKER for the URLRewrite approach)
The ghostty-web demo's client JS uses **hardcoded absolute paths**:
  - `import ... from '/dist/ghostty-web.js'`
  - `fetch('/api/token')`
  - `WebSocket('.../' + location.host + '/ws?...')`
So once the page loads at `/apps/terminal/`, the browser requests `/dist/*`, `/api/token`,
and `/ws` **at the site root**, not under `/apps/terminal`. The URLRewrite filter only
rewrites requests the client *sends to* `/apps/terminal/*` — which it never does. Result:
the HTML page loads (200) but its JS/WASM/token/WS requests 404 (they hit the dashboard's
`/` rule). **URLRewrite alone cannot make this absolute-pathed app work under a subpath.**

Options to proceed:
  1. **Dedicated hostname** (e.g. `term.apps01.co.slakin.net`) → `URLRewrite /` → ghostty.
     Cleanest; no app changes. (What "rewrite the backend URL ... for a dedicated hostname"
     may have intended.)
  2. **Patch the app to be subpath-aware**: change the 3 client paths to use a base derived
     from `location.pathname` (e.g. `/apps/terminal`), and set `GHOSTTY_ALLOWED_HOSTS`.
     Keeps the single host `apps01.co.slakin.net/apps/terminal`.
  3. Route the **whole** host to ghostty and move the dashboard elsewhere (not desired).

## Blocked on
- Docker access from the sandbox (group set, needs re-login / passwordless sudo / remote daemon)
  to `docker build` + `docker push`.
