# Requirements — Devpod (deploy-dashboard)

High-level requirements derived from the build history. Keep this file short and
unambiguous; link out for detail. A `TODO: <human review needed: ...>` marks a
genuine ambiguity that needs a decision.

## Identity & intent

A single-page web control panel for debugging Kubernetes workloads from inside
the cluster. One Go binary (stdlib only, no client-go) + one embedded HTML/JS
SPA. Users are authenticated **by the gateway**, not the app.

- Serve the SPA and a small JSON/WebSocket API for the `apps` namespace.
- The app's ServiceAccount must follow the
  [Kubernetes security model](kubernetes-security-model.md): cluster-scoped
  resources read-only; namespace-scoped resources feature-scoped.

## Functional requirements

### Deployments (the core tab)
- List deployments in `apps` **that carry a valid `workload-url` annotation**;
  all others are hidden from the UI.
- For each: name, replica/ready status, running state, and actions.
- **Toggle on/off** — scale to 0 (saving the prior count in an annotation
  `deploy-dashboard/last-replicas`) or restore.
- **View logs** — stream a pod's logs (WebSocket, auto-reconnect).
- **Terminate pods** — delete the deployment's pods (controller recreates them).
- **Delete deployment** — Advanced view only.
- **Workload link** — an external-style link from `workload-url`; must be
  same-origin relative (see security model / SEC-06).
- **Workload icon** — a `workload-icon` annotation rendered as a Primer Octicon.

### Jobs tab (Advanced view)
- List batch Jobs in `apps`, with a derived status (succeeded / pending /
  running / failed) from conditions/counts.
- **View job output** — reuse the log modal for the job's pod.

### Cluster nodes tab (Advanced view)
- List nodes with role, Ready state, and CPU/RAM utilization bars (from
  `metrics.k8s.io`; degrade to "—" when metrics-server is absent).

### User settings
- Store an SSH public key and up to N access tokens, persisted in the
  `user-settings` **Secret** (never a ConfigMap).
- Settings survive reloads (read on startup, written on change).

### View modes (persisted in user settings)
- **Basic** (default): only the Deployments tab. Delete-deployment action hidden.
- **Advanced**: Deployments + Jobs + Cluster nodes; delete-deployment visible.
- Theme (light/dark) is also a persisted user setting.

## Non-functional / platform requirements

- **Subpath-safe.** Must work when served under any path (e.g. `/app/`,
  `/u/<user>/`). Relative API calls + a backend `--base-path` flag + dynamic
  WebSocket URL.
- **Auth at the ingress layer.** No in-app login. Envoy Gateway
  `SecurityPolicy` (OIDC+JWT) gates all traffic. See `manifests/05-security-policy.yaml`.
- **TLS at the gateway** (Let's Encrypt); the app serves plain HTTP on its
  Service port.
- **Graceful 50x page.** A gateway `directResponse` status page at the root
  polls the app's `/healthz` and reloads the SPA when the backend recovers.
- **Static binary** (`CGO_ENABLED=0`) for the in-cluster image.
- **Primer-only frontend.** UI components and icons must be Primer CSS +
  Octicons (verbatim). See `docs/styleguide.md`.

## Explicit non-goals

- No multi-namespace support (pinned to `apps`).
- No in-app authentication or session/token issuance.
- No creation/editing of workloads from scratch — only scale, delete, and read.
  TODO: <human review needed: should users be able to *create* new deployments
  from the UI, or is the app strictly manage-existing-workloads-only? (The RBAC
  currently grants `create` on deployments as headroom, but no UI uses it.)>

## Open questions

- TODO: <human review needed: ambiguity on: access-token count/shape. The UI has
  3 fixed token slots; is that a hard limit or should it be unbounded key/value?
  Also: what are these tokens *for* (out-of-band API use? nothing currently
  consumes them server-side).>
- TODO: <human review needed: ambiguity on: per-user instances. The logout link
  and subpath design imply a possible `/u/<user>/` multi-tenant layout with one
  dashboard+terminal per user, but no operator/controller for that exists yet.
  Is that in scope or a future phase?>
