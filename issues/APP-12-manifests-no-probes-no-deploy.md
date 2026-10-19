# APP-12: Manifests lack resource limits, probes, and controller

- **Type:** Anti-pattern
- **Summary:** The dashboard container has no `resources` (limits/requests), no probes, and no controlling Deployment (see APP-02/APP-03). A single misbehaving process has no CPU/memory ceiling in the namespace.
- **Severity:** Medium
- **Effort:** Low

## Why this issue should be addressed / ranked
Complements APP-02/APP-03. Without requests/limits the Pod's QoS is Burstable/BestEffort and it can be OOM-killed at the wrong time or starve neighbors. For a long-lived server on a shared cluster this matters.

## Justification for the severity level
Medium. Scheduling/eviction behavior and namespace stability. Not correctness, but it affects reliability on a multi-tenant node.

## Dependencies
- **Depends on:** APP-02 (Deployment) so limits/probes live in the right place.
- **Enables:** nothing.

## Justification for the effort level
Low. Add `resources` (e.g., requests 50m/64Mi, limits 300m/256Mi) and the probes from APP-03. A few manifest lines.
