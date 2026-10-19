# SEC-09: Enforce OIDC auth via an Envoy Gateway SecurityPolicy

- **Type:** Security (implementation / follow-up)
- **Status:** IMPLEMENTED (manifest authored, dry-run validated; not yet applied live)
- **Summary:** Add an Envoy Gateway `SecurityPolicy` that enforces OIDC (dex)
  authentication in front of the `eg` Gateway, so every request to
  `apps01.co.slakin.net` is authenticated before reaching the
  `deploy-dashboard` Service. This is the mechanism that justifies marking
  **SEC-01** as WONT FIX (in-app auth is out of scope; auth is enforced at the
  ingress layer).
- **Severity:** Critical (this *is* the fix for the root vulnerability)
- **Effort:** Low (config only; no app code changes)

## Relationship to SEC-01
SEC-01 (unauthenticated endpoints) was closed as **WONT FIX in-app** because
the dashboard sits behind a gateway `SecurityPolicy` that enforces OIDC+JWT.
This issue is the concrete implementation of that decision. Once the
`SecurityPolicy` is applied, the "anyone with network reach" surface that made
SEC-01 Critical is closed.

## What was implemented
`manifests/05-security-policy.yaml` (two documents, namespace `apps`):

1. **`Secret/apps01-oidc-client`** (Opaque) — the dex client credentials,
   stored under the keys EG requires: `client-id: apps01-dev`,
   `client-secret: apps01devsecret`.
2. **`SecurityPolicy/apps01-oidc`** (`gateway.envoyproxy.io/v1alpha1`) —
   targets `Gateway/eg` + `sectionName: http` and enables the first-class
   **OIDC** provider:
   - `provider.issuer: https://dex.co.slakin.net/auth` — **must match the
     `issuer` value from dex's well-known config exactly.** Fetched live from
     `https://dex.co.slakin.net/auth/.well-known/openid-configuration`, which
     reports `"issuer": "https://dex.co.slakin.net/auth"` (note the `/auth`
     path — NOT the bare host). EG validates the JWT `iss` claim against this.
     Endpoints (authorization, token, end-session) are auto-discovered from the
     well-known URL.
   - `clientID: apps01-dev`, `clientSecret: {name: apps01-oidc-client}`.
   - `redirectURL: http://apps01.co.slakin.net/callback` — matches the dex
     client's registered `redirectURIs` (the EG default would be
     `/oauth2/callback`, so it is pinned explicitly).
   - `scopes: [openid, email, profile]`, `refreshToken: true`,
     `logoutPath: /logout`.
   - `passThroughAuthHeader: true` — lets non-browser clients (curl,
     `kubectl port-forward`, service-to-service) present
     `Authorization: Bearer <token>` and skip the redirect flow. Without it,
     the dashboard's own kubectl-based verification and any scripted client
     would get a 302 to dex.

## Why OIDC (spec.oidc), not JWT (spec.jwt)
- The registered `redirectURIs` (`…/callback`) signal a **browser login flow**,
  which is exactly what the first-class OIDC provider implements (redirect,
  cookie session, auto-refresh, logout). A bare `spec.jwt` provider only
  validates a presented Bearer token against a JWKS — it performs no login.
- Confirmed the deployed controller supports it: **Envoy Gateway v1.6.3**,
  `SecurityPolicy/v1alpha1`, and the live CRD schema includes both `oidc`
  (with `passThroughAuthHeader`) and `jwt`.

## Verification performed
- `kubectl apply --dry-run=server -f manifests/05-security-policy.yaml` →
  `secret/apps01-oidc-client created (server dry run)`,
  `securitypolicy.gateway.envoyproxy.io/apps01-oidc created (server dry run)`.
  Fields are valid against the live v1.6.3 CRD.
- Dex issuer + endpoints reachable from the cluster (well-known config fetched
  successfully).

## Status: NOT applied live (by design)
The manifest was authored and dry-run validated but intentionally **not
applied**, because:
- Enabling it makes the dashboard require a dex browser login, and the login
  (redirect → dex → `/callback`) cannot be completed headlessly in this
  environment.
- Applying it would make the dashboard's own kubectl/`curl`-based checks return
  302 redirects (mitigated for Bearer clients by `passThroughAuthHeader`, but
  the browser flow still needs a human).

**To enable:**
```
kubectl -n apps apply -f manifests/05-security-policy.yaml
```
Then a browser hitting `http://apps01.co.slakin.net/` is redirected to dex,
authenticates, returns on `/callback`, and is served the dashboard.

## Open items / follow-ups (blockers to full hardening)
1. **HTTP (not HTTPS) redirect.** The registered `redirectURI` is `http://…`
   and the current `eg` listener is plain HTTP (the HTTPS listener still has
   the `filter_chain_not_found` issue from earlier). The flow works over HTTP,
   but OIDC cookies will not be marked `Secure`. Once the HTTPS listener is
   fixed: change `redirectURL` to `https://apps01.co.slakin.net/callback` and
   re-register that URI on the `apps01-dev` dex client.
2. **Dev secret committed in plaintext.** `apps01devsecret` is in the manifest
   (flagged in a comment). Fine for a lab; for anything real, rotate it and
   keep it only in a K8s Secret, referencing it via `clientIDRef` /
   `clientSecret`.
3. **Authorization (not just authentication).** This enforces *who is calling*.
   It does not itself resolve SEC-03/SEC-04 (ConfigMap RBAC breadth, plaintext
   tokens) or per-user scoping. If per-user actions are wanted, map the OIDC
   identity (e.g. `email`/`sub` claim → header via `claimToHeaders` on a JWT
   provider, or an `Authorization` rule) and gate the app's write endpoints on
   it.

## Dependencies
- **Depends on:** the `eg` Gateway + HTTPS listener for `apps01.co.slakin.net`
  (HTTP today; HTTPS preferred).
- **Enables:** closing SEC-01's blast radius; optional per-user scoping.
