# APP-06: No server-level request timeouts

- **Type:** Anti-pattern
- **Summary:** `http.Server` is created with no `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout`. Slowloris-style or stuck client connections can hold goroutines and file descriptors indefinitely.
- **Severity:** Medium
- **Effort:** Low

## Why this issue should be addressed / ranked
Standard hardening that is easy to miss in a hand-rolled server. Without timeouts a slow or half-open client ties up a goroutine for its whole lifetime, which (with many clients) exhausts resources.

## Justification for the severity level
Medium. Availability/DoS resistance. No data risk, but it is a known foot-gun for long-lived servers, especially one that also holds long-lived WebSocket connections.

## Dependencies
- **Depends on:** none. Care needed so the timeouts do not kill intentional long-lived WS connections (they apply to the HTTP *request* lifecycle; the hijacked WS conn is separate, so this is safe).
- **Enables:** nothing.

## Justification for the effort level
Low. Set sensible timeouts on the `http.Server` struct (e.g., `ReadHeaderTimeout: 10s`, `IdleTimeout: 120s`). A few lines, low risk.
