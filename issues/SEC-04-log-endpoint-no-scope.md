# SEC-04: Log endpoint exposes any pod in the namespace, no filtering

- **Type:** Security
- **Summary:** `/api/pods` and the log stream (`/api/logs` HTTP and `/api/logs-ws` WebSocket) accept a `pod` parameter and will stream **any** pod in the namespace, with no ownership check and no per-line redaction. Combined with SEC-01 this is unauthenticated log access.
- **Severity:** High
- **Effort:** Medium

## Why this issue should be addressed / ranked
Logs are the richest source of accidental secret leakage (DB URLs, tokens printed at startup, request payloads). The app exposes the entire namespace's logs to whoever holds SEC-01's reach. It is ranked high because the data disclosed is sensitive even without any write action.

## Justification for the severity level
High. Information disclosure of live application logs. Not "Critical" only because it is read-only and already bounded to the `apps` namespace; but it is the most likely way credentials or PII leak out of this system.

## Dependencies
- **Depends on:** SEC-01 for full mitigation (gate the endpoint).
- **Enables:** a per-user "can view my deployments" model if authentication adds ownership. Independent of SEC-02/03.

## Justification for the effort level
Medium. Minimum fix is gating the endpoints (covered by SEC-01) plus validating the pod parameter against the known set and, ideally, rate-limiting the WebSocket. Optional secret-redaction of log lines adds parsing complexity, which is what pushes this from Low to Medium.
