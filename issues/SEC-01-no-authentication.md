# SEC-01: No authentication on any endpoint

- **Type:** Security
- **Summary:** Every HTTP route (read and write) is unauthenticated. Anyone who can reach the Service (ClusterIP, NodePort, or via the Envoy Gateway) can read deployments/pods, scale workloads, delete deployments, and read all logs.
- **Severity:** Critical
- **Effort:** High

## Why this issue should be addressed / ranked
It is the root vulnerability: without authentication, every other exposure (log disclosure, destructive writes, config read/write) is directly reachable by anyone with network reach. No amount of network-level patching fully closes it in a cluster whose nodes are on a shared LAN, so it is the highest-ranked security item.

## Justification for the severity level
Critical. The blast radius is a full write path to the `apps` namespace (scale to 0, delete deployments) plus a full read path (all pod logs, which routinely contain secrets). There is no credential check anywhere in `cmd/main.go`. Tolerable in a closed lab; a serious breach vector anywhere shared.

## Dependencies
- **Depends on:** none (foundational).
- **Enables:** proper scoping for SEC-03 (config), SEC-04 (tokens). Network-level hardening (NetworkPolicy / Ingress auth) is a complement, not a replacement.

## Justification for the effort level
High. Real options (OIDC, a reverse-proxy auth middleware, or a shared-secret + per-user token scheme with a small auth store) each require designing the login flow, session/token handling, and wiring a middleware in front of every handler, plus RBAC for any backing identity store. It is a design-and-integration task, not a one-liner.
