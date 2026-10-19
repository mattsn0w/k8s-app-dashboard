# APP-03: No readiness or liveness probes

- **Type:** Anti-pattern
- **Summary:** The container spec defines no `readinessProbe` or `livenessProbe`. The Service can route traffic to a not-ready instance, and a wedged process is never restarted.
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
