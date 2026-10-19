# APP-03: No readiness or liveness probes

- **Type:** Anti-pattern
- **Status:** FIXED
- **Summary:** The container spec defined no `readinessProbe` or `livenessProbe`. The Service can route traffic to a not-ready instance, and a wedged process is never restarted.
- **Severity:** Medium
- **Effort:** Low

## Why this issue should be addressed / ranked
Directly tied to APP-02. Without a readiness probe the Service sends requests during cold start; without liveness a hung goroutine or TLS stall is never self-healed. For a long-running HTTP + WebSocket server these are table stakes.

## Justification for the severity level
Medium. Availability, not correctness. The app has no dedicated `/healthz` endpoint today, so this also implies adding a trivial health route.

## Dependencies
- **Depends on:** APP-02 (probes live in the Deployment container spec).
- **Enables:** safer rollout of APP-02.

## Justification for the effort level
Low. Add a `GET /healthz` (returns 200, no K8s call needed), then `readinessProbe` + `livenessProbe` httpGet on it. Small Go route + a few manifest lines.

## Resolution (implemented) — Status: **FIXED**

The `deploy-dashboard` container in `manifests/03-deployment.yaml` now has
both probes, pointing at the `/healthz` route the binary already serves
(`handleHealthz` → `200 ok`, no K8s call):

- `readinessProbe`: `httpGet /healthz:8080`, `initialDelaySeconds: 3`,
  `periodSeconds: 10`, `failureThreshold: 3` — gates Service traffic during
  cold start.
- `livenessProbe`: `httpGet /healthz:8080`, `initialDelaySeconds: 10`,
  `periodSeconds: 20`, `failureThreshold: 3` — restarts a wedged process.

Note: the issue's "add a trivial health route" prerequisite was already met —
`cmd/main.go` registered `mux.HandleFunc("/healthz", c.handleHealthz)`. Only
the manifest probe lines were missing. Verified live: pod reports
`Ready=True` / `ContainersReady=True` and the Service endpoint is populated.
