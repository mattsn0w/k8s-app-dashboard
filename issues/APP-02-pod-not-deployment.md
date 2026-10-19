# APP-02: Dashboard runs as a raw Pod, not a Deployment

- **Type:** Anti-pattern
- **Status:** FIXED
- **Summary:** `manifests/03-deployment.yaml` created a `Pod` (`dashboard-runner`), not a `Deployment`. There is no controller to restart it on crash or node drain, no rolling updates, and no replica management.
- **Severity:** Medium
- **Effort:** Low

## Why this issue should be addressed / ranked
A raw Pod is not the unit K8s is designed to keep alive. If the node (`brick3`/`brick6`) drains or the container restarts, nothing reschedules or restarts it, and "deploying a new version" means deleting the Pod and re-applying a spec — exactly the manual churn seen during development.

## Justification for the severity level
Medium. Reliability and ops-hygiene, not correctness. The app works while the node is up, but the platform guarantees (restart, self-heal, rolling update) are missing.

## Dependencies
- **Depends on:** none.
- **Enables:** APP-03 (probes) attach to a Deployment container spec; do both together.

## Justification for the effort level
Low. Convert the Pod spec to a `Deployment` (1 replica, same container/env/volume), add a matching Service selector. The Service already points at the right label, so this is mostly a re-wrap, not a rewrite.

## Resolution (implemented) — Status: **FIXED**

`manifests/03-deployment.yaml` now creates a `Deployment` (`deploy-dashboard`,
`replicas: 1`, `strategy: Recreate`) instead of a raw `Pod`. The old
`dashboard-runner` Pod was deleted from the cluster and the Service (unchanged
selector `app.kubernetes.io/name=deploy-dashboard`) now routes to the
Deployment's pod.

Key decisions:
- **`strategy: Recreate`** — with an injected binary in an `emptyDir` there is
  no image to roll, and a second replica would have no binary to start, so
  `Recreate` is correct (avoids a broken rolling window).
- **`initContainer: wait-for-binary`** — blocks until the static binary is
  injected into the shared `emptyDir`, so the main container never execs a
  missing `/app/deploy-dashboard`. This is what makes the `emptyDir` wipe on
  pod recreation safe (see `build-and-deploy.sh`).
- **Probes added** (also closes APP-03): `readinessProbe` + `livenessProbe`
  both `httpGet /healthz:8080`, which the binary already exposes.
- **`build-and-deploy.sh`** rewritten to the Deployment flow: apply the
  manifest → find the `pod/deploy-dashboard-*` → `tar | kubectl exec` the
  binary into the initContainer's shared volume → verify SHA256 → wait for
  readiness.

Verified live: `kubectl -n apps get deploy deploy-dashboard` → `1/1
AVAILABLE`; single pod `1/1 Running`, `Ready=True`, `ContainersReady=True`;
Service endpoint `10.42.1.128:8080`; `/` serves the embedded Devpod UI;
`/healthz` → `ok`; `/api/deployments` returns data (and now lists the
`deploy-dashboard` Deployment itself). The old `dashboard-runner` Pod is gone.


## The new spec (`manifests/03-deployment.yaml`)

The raw `Pod` document was replaced by this `apps/v1 Deployment`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy-dashboard
  namespace: apps
  labels:
    app.kubernetes.io/name: deploy-dashboard
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: deploy-dashboard
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app.kubernetes.io/name: deploy-dashboard
    spec:
      serviceAccountName: deploy-dashboard
      initContainers:
      - name: wait-for-binary
        image: debian:bookworm-slim
        command: [sh, -c]
        args:
        - |
          echo "init: waiting for /app/deploy-dashboard to be injected..."
          while [ ! -f /app/deploy-dashboard ]; do sleep 3; done
          echo "init: binary present, proceeding"
        volumeMounts:
        - name: app
          mountPath: /app
      containers:
      - name: dashboard
        image: debian:bookworm-slim
        command: [sh, -c]
        args:
        - |
          echo "Starting deploy-dashboard..."
          exec /app/deploy-dashboard --listen=:8080 --namespace=apps
        volumeMounts:
        - name: app
          mountPath: /app
        ports:
        - containerPort: 8080
          name: http
        readinessProbe:
          httpGet: { path: /healthz, port: 8080 }
          initialDelaySeconds: 3
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3
        livenessProbe:
          httpGet: { path: /healthz, port: 8080 }
          initialDelaySeconds: 10
          periodSeconds: 20
          timeoutSeconds: 3
          failureThreshold: 3
      volumes:
      - name: app
        emptyDir: {}
```

The `Service` (selector `app.kubernetes.io/name=deploy-dashboard`) is
unchanged — it already matched the label, so it now routes to the
Deployment's pod with no edit. The Gateway/HTTPRoute/ReferenceGrant documents
in the same file are unchanged.

## `build-and-deploy.sh` flow (Deployment-aware)

1. `CGO_ENABLED=0 go build ... -o /tmp/deploy-dashboard-static ./cmd/`
   (UI embedded via `//go:embed` — see APP-01) and record its SHA256.
2. `kubectl apply -f manifests/03-deployment.yaml` — creates/updates the
   Deployment; the pod it spawns is stuck in `Init:0/1` (initContainer
   blocking on the missing binary).
3. Discover the pod by name prefix: `kubectl -n apps get pods -o name |
   grep '^pod/deploy-dashboard-'` (the old raw `dashboard-runner` name no
   longer exists, so this uniquely targets the Deployment's pod).
4. Inject: `tar cf - -C /tmp deploy-dashboard-static | kubectl -n apps
   exec -i <pod> -c wait-for-binary -- tar xf - -C /app/`, then
   `mv /app/deploy-dashboard-static /app/deploy-dashboard && chmod +x`. The
   init loop sees the file and the main container starts.
5. **Verify SHA256** in-pod matches local (byte-for-byte; `kubectl cp` is
   avoided because it breaks with "broken pipe" on >1 MB binaries).
6. `kubectl -n apps wait --for=condition=Ready pod/<pod> --timeout=120s`.
7. Port-forward the **Service** and hit `/healthz`, `/`, `/api/deployments`.

## Verification matrix (live)

| Check | Command | Result |
|---|---|---|
| Deployment available | `kubectl -n apps get deploy deploy-dashboard` | `1/1 UP-TO-DATE 1 AVAILABLE` |
| Pod running + ready | `kubectl -n apps get pod <dep-pod>` | `1/1 Running`, 0 restarts |
| Pod conditions | `... -o jsonpath='{.status.conditions[*]}'` | `Ready=True`, `ContainersReady=True` |
| Service endpoint | `kubectl -n apps get endpoints deploy-dashboard` | `10.42.1.128:8080` (single) |
| Repo == cluster | `kubectl diff -f manifests/03-deployment.yaml` | no differences |
| Frontend | `GET /` (via port-forward) | `<title>Devpod</title>`, Primer CSS |
| Health | `GET /healthz` | `ok` |
| API | `GET /api/deployments` | JSON incl. `deploy-dashboard` itself |

## Operational notes
- The old raw `Pod/dashboard-runner` was deleted; only the Deployment-managed
  pod remains under the shared label.
- `strategy: Recreate` is deliberate: a rolling update would spawn a 2nd
  replica that has no injected binary (emptyDir is per-pod), which would
  block in its init container indefinitely. `Recreate` tears down then
  recreates, and `build-and-deploy.sh` re-injects the binary each run.
