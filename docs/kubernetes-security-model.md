# Kubernetes security model

How `deploy-dashboard` scopes its Kubernetes access. The ServiceAccount
`deploy-dashboard` (namespace `apps`) is the only identity the app uses.

## The two objects: ClusterRole (read-only) + Role (namespaced CRUD)

The app's RBAC is split so the **scope of each grant matches its blast radius**:

- **ClusterRole `deploy-dashboard-cluster`** — holds *only* the genuinely
  cluster-scoped resources (`nodes`, `metrics.k8s.io/nodes`), **read-only**.
  These resources are cluster-wide by nature, so a ClusterRole is the only thing
  that can grant them; the app only ever reads node/metrics data.
- **Role `deploy-dashboard`** (namespace `apps`) — holds *all* namespace-scoped
  resources with their feature verbs. A **Role's rules apply only in its own
  namespace**, so this confines every namespaced grant to `apps`.

This split is the core security property. It is **wrong** to put the namespaced
resources in a ClusterRole: a ClusterRole's rules apply to **every** namespace,
and `subjects[].namespace` on a ClusterRoleBinding scopes only *which
ServiceAccount gets the binding* — **not** which namespaces the rules apply in.
So a ClusterRole granting `get`/`delete` on `pods` would let the SA read/delete
pods in *all* namespaces, not just `apps`. (Demonstrated by
`manifests/rbac-scope-test.yaml`.)

Verify scope of any resource:

```bash
kubectl api-resources | head -n1              # column header (NAMESPACED)
kubectl api-resources | grep ' false '        # cluster-scoped (NAMESPACED=false)
kubectl api-resources | grep ' true '         # namespace-scoped (NAMESPACED=true)
```

### Cluster-scoped grants (read-only)

| Resource | Scope | Verbs | Why |
|---|---|---|---|
| `nodes` (v1) | cluster | `get,list,watch` | Nodes tab: names, roles, Ready state. |
| `nodes` (metrics.k8s.io) | cluster | `get,list` | Per-node CPU/RAM bars. Degrades to "—" if metrics-server absent. |

### Namespace-scoped grants (confined to `apps`)

Namespace-scoped resources get the verbs a feature actually needs — nothing
more. All live in the `apps` Role, so they apply **only** in `apps`.

| Resource | Scope | Verbs | Feature that needs them |
|---|---|---|---|
| `pods` | namespace | `get,list,watch,delete` | List pods; **Terminate pods** (delete a pod to force-restart). |
| `pods/log` | namespace | `get` | Log stream (`?follow=true`, over HTTP + WebSocket). |
| `services` | namespace | `get,list,watch` | Workload metadata. Read-only — no feature writes Services. |
| `deployments` | namespace | `get,list,watch,create,update,patch,delete` | Scale (patch), delete, create (headroom). The core CRUD surface. |
| `replicasets` | namespace | `get,list,watch,update,patch` | Replica status display + scale headroom. `update`/`patch` keep the scale-down-to-zero / scale-back-up capability robust (scaling is normally done on the Deployment, which cascades to the ReplicaSet). |
| `jobs` (batch) | namespace | `get,list,watch` | Jobs tab: list + view output. Read-only. |
| `secrets` (`user-settings` only) | namespace | `get,update,patch` | SSH key + tokens. Name-scoped via `resourceNames` (see below). |

## Least-privilege specifics

- **Name-scoped Secret.** The user-settings grant uses
  `resourceNames: ["user-settings"]`, so the SA can only touch that one named
  Secret — no `list` over all Secrets, no `create`. (Fixes SEC-02/SEC-03.)
- **Subresource isolation.** `pods/log` is granted separately from `pods`;
  reading logs does not imply mutating pods.
- **Read-only where the feature is read-only.** `services`, `replicasets`, and
  `jobs` are never written by the app, so they carry no write verbs. A future
  feature that needs to write one must add the verb deliberately.

## What the SA *cannot* do (by design)

- Mutate any cluster-scoped resource (nodes, namespaces, clusterroles, …).
- `list`/`get` Secrets other than `user-settings`.
- Create/delete Services, Jobs, or ReplicaSets.
- Read or write anything outside namespace `apps` (all request paths are
  pinned to `apps`).

## Defense in depth (independent of RBAC)

RBAC is the authorization boundary; these close other gaps the app can't:

- **Authentication at the gateway.** The app enforces *no* auth of its own —
  the Envoy Gateway `SecurityPolicy` (OIDC + JWT) authenticates every request
  before it reaches the Service. See `manifests/05-security-policy.yaml` and
  `issues/SEC-01` / `SEC-09`. An unauthenticated direct Service hit is out of
  the threat model.
- **Same-origin API (no CORS).** `sameOriginOnly` middleware strips any
  `Access-Control-Allow-Origin` so the JSON API can't be read cross-origin.
  See `issues/SEC-07`.
- **Secrets, not ConfigMaps, for credentials.** User SSH keys/tokens live in
  the `user-settings` Secret. See `issues/SEC-02`.
- **workload-url pinned to same-origin.** Prevents the dashboard from rendering
  an external/`javascript:` link. See `issues/SEC-06`.

## Scope of effect

Namespace-scoped grants apply **only** in `apps` (they are a Role in that
namespace). Cluster-scoped grants (nodes/metrics) are inherently cluster-wide
but read-only. The app also hard-codes `*namespace=apps` in every request path,
so even the read-only cluster grants are used only for cluster-level node data.
If the app is ever pointed at another namespace, add a second Role/RoleBinding
for that namespace rather than widening the existing one.

## Verifying the model

```bash
SA=system:serviceaccount:apps:deploy-dashboard
# Cluster-scoped must be read-only:
kubectl auth can-i create  nodes --as=$SA     # -> no
kubectl auth can-i patch   nodes --as=$SA     # -> no
kubectl auth can-i get     nodes --as=$SA     # -> yes
# Namespace-scoped in apps:
kubectl auth can-i patch   deployments --as=$SA -n apps   # -> yes
kubectl auth can-i patch   replicasets --as=$SA -n apps   # -> yes  (scale headroom)
kubectl auth can-i get     pods/log      --as=$SA -n apps  # -> yes
# The key property: namespaced grants do NOT leak to other namespaces:
kubectl auth can-i get     pods        --as=$SA -n default      # -> no
kubectl auth can-i delete   pods        --as=$SA -n kube-system # -> no
```
