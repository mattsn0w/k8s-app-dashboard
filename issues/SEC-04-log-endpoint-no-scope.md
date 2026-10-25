# SEC-04: Log endpoint exposes any pod in the namespace, no filtering

- **Type:** Security
- **Status:** WONT FIX (auth at the ingress layer; residual is a scoping/authorization concern)
- **Summary:** `/api/pods` and the log stream (`/api/logs` HTTP and `/api/logs-ws` WebSocket) accept a `pod` parameter and will stream **any** pod in the namespace, with no ownership check and no per-line redaction. Combined with SEC-01 this is unauthenticated log access.
- **Severity:** High
- **Effort:** Medium

## Why this issue should be addressed / ranked
Logs are the richest source of accidental secret leakage (DB URLs, tokens printed at startup, request payloads). The app exposes the entire namespace's logs to whoever holds SEC-01's reach. It is ranked high because the data disclosed is sensitive even without any write action.

## Justification for the severity level
High. Information disclosure of live application logs. Not "Critical" only because it is read-only and already bounded to the `apps` namespace; but it is the most likely way credentials or PII leak out of this system.

## Dependencies
- **Depends on:** SEC-01's resolution model — the Envoy Gateway `SecurityPolicy`
  (OIDC+JWT) authenticates every request *before* it reaches these endpoints,
  so the "unauthenticated log access" premise is closed at the ingress layer,
  the same reason SEC-01 is WONT FIX. No in-app change is made for that.
- **Residual (not addressed here):** *authorization/scoping* — which authenticated
  user may view which pod's logs — is a separate concern from "who is calling"
  and is out of scope for this issue, consistent with SEC-01.
- **Enables:** a per-user "can view my deployments" model if authorization adds ownership. Independent of SEC-02/03.

## Justification for the effort level
Medium. Minimum fix is gating the endpoints (covered by SEC-01) plus validating the pod parameter against the known set and, ideally, rate-limiting the WebSocket. Optional secret-redaction of log lines adds parsing complexity, which is what pushes this from Low to Medium.

## Status: **WONT FIX** — authentication is enforced at the ingress layer (inline with SEC-01)

The "unauthenticated log access" that made this High is closed by the **same**
mechanism that resolved SEC-01: the dashboard sits behind an Envoy Gateway
`SecurityPolicy` that enforces **OIDC + JWT validation**. No request reaches
`/api/pods`, `/api/logs`, or `/api/logs-ws` without first being authenticated at
the gateway. So, consistent with SEC-01, this is deliberately **not** fixed in
`cmd/main.go` — implementing per-pod ownership or secret-redaction in the app
would duplicate work the gateway boundary already provides for the
*authentication* half.

### What this does *not* close
- **Authorization/scoping** (which pod a given authenticated user may view) and
  **log-line redaction** remain potential hardening if a per-user ownership or
  data-classification model is ever required. These are separate from the
  gateway's identity check and would be re-opened as their own issues if needed.
- **Defense in depth:** if the dashboard is ever exposed outside the gateway
  (direct Service/NodePort, a second ingress, or an internal caller that skips
  the `SecurityPolicy`), the lack of in-app auth becomes real again — the same
  residual SEC-01 notes. Re-open accordingly.

### Severity note
The original "High" rating assumed unauthenticated reach (per the SEC-01
premise). Given the mandated gateway `SecurityPolicy` with OIDC+JWT, the
authentication gap is out of scope by design — hence **WONT FIX**, not "fixed in
the app." The remaining scoping/redaction work is optional hardening, tracked
separately if pursued.
