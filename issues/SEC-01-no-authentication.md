# SEC-01: No authentication on any endpoint

- **Type:** Security
- **Status:** WONT FIX
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


## Status: **WONT FIX** — authentication is enforced at the ingress layer

This is deliberately left to the **Envoy Gateway `SecurityPolicy`** rather
than implemented in the app. The dashboard sits behind an Envoy Gateway
`SecurityPolicy` that enforces **OIDC + JWT validation**: requests are
authenticated and the caller's identity is validated by the gateway *before*
traffic reaches the `deploy-dashboard` Service. The app therefore does not
implement its own login/session/token handling — that responsibility belongs
to the gateway, which already owns TLS termination, routing, and (via the
`SecurityPolicy`) authentication for this hostname.

### Why WONT FIX rather than fixing it in `cmd/main.go`
- **Single source of auth.** OIDC/JWT validation at the gateway is the
  cluster's standard identity boundary. Re-implementing it in Go would
  duplicate the login flow, session handling, and JWKS trust configuration,
  and would have to stay in sync with the gateway — two copies of the same
  security policy.
- **The app is a trusted-internal service.** Once the `SecurityPolicy`
  admits a request, the caller is already authenticated. The remaining
  concerns are *authorization/scoping* (SEC-03, SEC-04), which are separate
  from "who is calling at all."
- **Lower blast radius than assumed.** The "anyone with network reach"
  premise is closed by the `SecurityPolicy` (plus the `ReferenceGrant`/
  namespace isolation and ClusterIP-only Service). The raw unauthenticated
  surface only exists if someone bypasses the gateway, which is out of the
  app's threat model.

### What this does *not* close
- **SEC-03 / SEC-04** (over-broad ConfigMap RBAC, plaintext tokens) remain
  open — they are authorization/handling concerns that a gateway JWT does not
  itself resolve.
- **Defense in depth:** if the dashboard is ever exposed outside the gateway
  (direct Service/NodePort, a second ingress, or an internal caller that skips
  the `SecurityPolicy`), the lack of in-app auth becomes real again. Re-open
  this issue if that deployment path is introduced.

### Residual note
The original "Critical" severity is accurate *for an internet/shared-LAN
exposure with no fronting auth*. Given the mandated gateway `SecurityPolicy`
with OIDC+JWT, the in-app gap is out of scope by design — hence **WONT FIX**,
not "fixed in the app."

### Implementation
See **SEC-09** (`issues/SEC-09-oidc-securitypolicy.md`) for the concrete
`SecurityPolicy` (OIDC provider) in `manifests/05-security-policy.yaml` — the
ingress-layer mechanism this WONT-FIX decision relies on.
