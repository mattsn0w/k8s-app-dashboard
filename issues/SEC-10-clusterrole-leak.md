# SEC-10: ClusterRole leaked namespaced grants to every namespace

- **Type:** Security
- **Summary:** The app's RBAC was a single **ClusterRole** + **ClusterRoleBinding** for *all* resources (pods, services, deployments, replicasets, jobs, secrets) plus the cluster-scoped `nodes`. A ClusterRole's rules apply to **every** namespace, so `get/list/watch` on `pods`/`services`/`deployments` and `delete` on `pods` were granted cluster-wide — not just in `apps`. The `subjects[].namespace: apps` on the ClusterRoleBinding scoped only *which ServiceAccount got the binding*, not *which namespaces the rules applied in* (an anti-pattern).
- **Severity:** High
- **Effort:** Low
- **Status:** ✅ **FIXED** (2026-10-26)

## Why this issue should be addressed / ranked
The intended model is "users debug *their workloads* in `apps` from inside the cluster." A ClusterRole that grants `delete pods` and `patch deployments` gives the app's ServiceAccount that power in **every** namespace of the cluster — a far larger blast radius than the feature requires. Any pod/SA that can impersonate or share this SA could touch other tenants' or system workloads.

## The core RBAC principle (the lesson)
- **ClusterRole** = rules apply to **all** namespaces (and all cluster-scoped resources). Use it *only* for genuinely cluster-scoped resources, and keep those **read-only**.
- **Role** = rules apply **only** in the Role's own namespace. Use it for all namespace-scoped resources.
- `subjects[].namespace` on a **ClusterRoleBinding** is **not** a scope control — it only names the bound subject. It does *not* confine the ClusterRole's rules to that namespace.

## Demonstration (the test that proved it)
Two SAs in a throwaway namespace, both granted `get pods`:

| SA binding | `kubectl auth can-i get pods -A` | `kubectl auth can-i get pods` |
|---|---|---|
| **Role** + RoleBinding (namespaced) | **no** ✅ | **yes** ✅ |
| **ClusterRole** + ClusterRoleBinding | **yes** ⚠️ (the bug) | **yes** ✅ |

The Role is confined to its namespace; the ClusterRole leaks to all namespaces.
Runnable version: `tests/k8s-role-sec/` (see below).

## Fix
Split `manifests/02-rbac.yaml` into two objects so each grant's scope matches its blast radius:

- **ClusterRole `deploy-dashboard-cluster`** — *only* cluster-scoped resources, read-only:
  `nodes` (v1) `get,list,watch`; `nodes` (metrics.k8s.io) `get,list`.
- **Role `deploy-dashboard`** (namespace `apps`) — all namespace-scoped resources with
  feature verbs (pods, pods/log, services, deployments, replicasets, jobs, and the
  name-scoped `user-settings` Secret).

`replicasets` carries `update,patch` so the scale-down-to-zero / scale-back-up
capability is robust regardless of which object the scale path touches (scaling is
normally done on the Deployment, which cascades to the ReplicaSet).

Verified with `kubectl auth can-i`: namespaced verbs are **denied** in
`default`/`kube-system` and **allowed** in `apps`; cluster-scoped `nodes` stays
read-only.

## Runnable tests
`tests/k8s-role-sec/` contains a self-contained check you can run against any
cluster:
- `run.sh` — applies a fixture (SA + Role/RoleBinding + a pod with the SA's
  in-cluster kubeconfig), then asserts:
  1. the SA **can** perform the required verbs on namespaced resources in its
     namespace, and
  2. the SA is **denied** on cluster-scoped resources (e.g. `get nodes`,
     `create namespaces`).
- `fixture.yaml` — the test fixtures (namespace, SA, Role, RoleBinding, pod).

Run: `tests/k8s-role-sec/run.sh` (needs `kubectl` + a kubeconfig that can create
the fixture namespace). Exits 0 on pass, 1 on any failure.

## References
- [RBAC authorization — Role vs ClusterRole](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Use RBAC for authorization](https://kubernetes.io/docs/tasks/authenticate-access/control-access-to-kubernetes-api/using+rbaac/)
- `docs/kubernetes-security-model.md` — the app's full scope model.
- `manifests/02-rbac.yaml` — the fixed grants.
