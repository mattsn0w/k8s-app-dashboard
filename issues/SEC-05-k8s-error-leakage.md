# SEC-05: Raw K8s API error text returned to the client

- **Type:** Security
- **Summary:** Handlers pass K8s API error strings (and HTTP status text) straight to the browser via `writeError`/`http.Error`. These can include internal object names, the requesting ServiceAccount, field-level API paths, and other cluster internals.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
Information-disclosure hygiene. Low because it leaks metadata, not credentials, and is only reachable behind SEC-01. Ranked in the low band but included because the fix is a single helper and it improves both security and UX at once.

## Justification for the severity level
Low. The disclosed strings are cluster metadata (names, API groups, reason codes), not secrets. Impact is mainly aiding reconnaissance for an attacker who already has read access.

## Dependencies
- **Depends on:** none.
- **Enables:** better error UX (see UX items) — a sanitized generic message with a logged full detail on the server is the natural home for a logging change too.

## Justification for the effort level
Low. Add a small mapping (status code → generic client message) and `log.Printf` the full error server-side. One helper, no behavioral risk.
