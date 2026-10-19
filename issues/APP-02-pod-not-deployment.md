# APP-02: Dashboard runs as a raw Pod, not a Deployment

- **Type:** Anti-pattern
- **Summary:** `manifests/03-deployment.yaml` creates a `Pod` (`dashboard-runner`), not a `Deployment`. There is no controller to restart it on crash or node drain, no rolling updates, and no replica management.
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
