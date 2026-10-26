# k8s-role-sec — RBAC scope test (SEC-10)

Runnable check that a namespace-scoped **Role** grants the needed verbs on
namespaced resources *only* in its own namespace and gives **no** cluster-scoped
access. Guards against the anti-pattern of putting namespaced grants in a
ClusterRole (which leaks them to every namespace).

## Files
- `run.sh` — applies `fixture.yaml`, runs `kubectl auth can-i` assertions, prints
  PASS/FAIL, and cleans up. Exits 0 on all-pass, 1 on any failure.
- `fixture.yaml` — a namespace + ServiceAccount + Role + RoleBinding (namespaced
  grants only; no ClusterRole).

## Run
```sh
tests/k8s-role-sec/run.sh          # runs and tears down the fixtures
KEEP=1 tests/k8s-role-sec/run.sh   # leave the fixtures for inspection
```

Needs `kubectl` and a kubeconfig that can create the `rbac-sec-test` namespace.

## What it asserts
1. **Namespaced, allowed** — the SA can `get/list/watch/delete` pods, read
   `pods/log`, read services, full CRUD on deployments, and `update/patch`
   replicasets, all *in its own namespace*.
2. **Cluster-scoped, denied** — the SA is denied on `nodes`, `namespaces`,
   `clusterroles`, `clusterrolebindings` (it has no ClusterRole).
3. **No cross-namespace leak** — the same namespaced resources are denied in
   `default` and `kube-system`.

See `issues/SEC-10-clusterrole-leak.md` for the full story and the
Role-vs-ClusterRole rationale.
