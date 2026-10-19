# APP-05: No rate limiting on any endpoint

- **Type:** Anti-pattern
- **Summary:** No handler throttles. `GET /api/deployments` triggers a full K8s API fan-out (list deployments + list pods + per-deployment pod mapping) on every call, and the frontend polls it every 10 s. An unauthenticated caller (SEC-01) can drive the K8s API hard.
- **Severity:** Medium
- **Effort:** Medium

## Why this issue should be addressed / ranked
The polling + fan-out pattern means load on the API server scales with (clients × poll rate), and nothing bounds it. It is an operational-robustness gap that becomes a security amplification once SEC-01 exists.

## Justification for the severity level
Medium. It degrades the cluster's API server under load/abuse rather than leaking data. Important for a shared cluster, not a correctness bug.

## Dependencies
- **Depends on:** none (mitigated in practice by SEC-01).
- **Enables:** nothing.

## Justification for the effort level
Medium. Requires a per-IP/per-token limiter (e.g., a token bucket or `golang.org/x/time/rate`) wired into a middleware, plus deciding limits and 429 responses. More than a one-liner because the WS endpoints need their own handling.
