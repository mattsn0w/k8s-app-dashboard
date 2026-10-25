# SEC-05: Raw K8s API error text returned to the client

- **Type:** Security
- **Status:** WONT FIX (auth at the ingress layer; disclosure is bounded to already-authenticated callers)
- **Summary:** Handlers pass K8s API error strings (and HTTP status text) straight to the browser via `writeError`/`http.Error`. These can include internal object names, the requesting ServiceAccount, field-level API paths, and other cluster internals.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
Information-disclosure hygiene. Low because it leaks metadata, not credentials, and is only reachable by callers who are **already authenticated at the Envoy Gateway `SecurityPolicy`** (the same ingress-layer auth that resolved SEC-01). Ranked in the low band but included because the fix is a single helper and it improves both security and UX at once.

## Justification for the severity level
Low. The disclosed strings are cluster metadata (names, API groups, reason codes), not secrets. Impact is mainly aiding reconnaissance for an attacker who already has read access.

## Dependencies
- **Depends on:** SEC-01's resolution model — the Envoy Gateway `SecurityPolicy`
  (OIDC+JWT) means only authenticated callers can trigger these error responses,
  so the audience for the leaked metadata is already trusted. Same reason SEC-01
  is WONT FIX; no in-app change is made for the authentication half.
- **Enables:** better error UX (see UX items) — a sanitized generic message with a logged full detail on the server is the natural home for a logging change too.

## Justification for the effort level
Low. Add a small mapping (status code → generic client message) and `log.Printf` the full error server-side. One helper, no behavioral risk.

## Status: **WONT FIX** — disclosure is bounded to authenticated callers (inline with SEC-01)

The audience for the leaked error text is limited by the **same** ingress-layer
auth that resolved SEC-01: the Envoy Gateway `SecurityPolicy` enforces
**OIDC + JWT validation**, so only an already-authenticated caller can reach a
handler that emits a raw K8s error. The disclosed strings are cluster metadata
(names, API groups, reason codes), not credentials, and they go to a caller the
gateway has already vetted. Consistent with SEC-01, this is deliberately **not**
reworked in `cmd/main.go` for the authentication half.

### What this does *not* close
- **Sanitization / better UX** (generic client message + server-side logging of
  the full detail) remains a reasonable, independent improvement tracked under
  the UX items. It is hardening/UX, not the authentication gap this issue was
  filed under.
- **Defense in depth:** if the dashboard is ever exposed outside the gateway
  (direct Service/NodePort, a second ingress, or an internal caller that skips
  the `SecurityPolicy`), the raw-error disclosure becomes meaningful again — the
  same residual SEC-01 notes. Re-open accordingly.

### Severity note
The "Low" rating already assumed a trusted audience. Given the mandated gateway
`SecurityPolicy` with OIDC+JWT, that audience is enforced at the ingress layer —
hence **WONT FIX** for the security framing, not "fixed in the app." Any
sanitization work is optional UX/hardening, tracked separately if pursued.
