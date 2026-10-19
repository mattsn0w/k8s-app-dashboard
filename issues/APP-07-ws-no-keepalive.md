# APP-07: No server-sent keepalive/ping on the WebSocket

- **Type:** Anti-pattern
- **Summary:** The server replies to client pings (opcode `0x9`→`0xA`) but never initiates its own ping/keepalive. A silently-dead TCP connection (no FIN) is only discovered on the next read/write, so a wedged client can hold the K8s log pump goroutine indefinitely.
- **Severity:** Low
- **Effort:** Medium

## Why this issue should be addressed / ranked
Each streaming connection runs a goroutine reading the K8s log stream. If the client vanishes without a FIN (laptop sleep, NAT drop), that goroutine and its upstream K8s log stream can leak until a write finally fails. Self-pings bound that window.

## Justification for the severity level
Low. It is a leak under unusual network conditions, not a crash or data issue, and the 1 MiB cap + auth (SEC-01) limit exposure. But it is the kind of bug that shows up as slow goroutine growth in production.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing. Pairs naturally with APP-08 (read deadlines) — a ping also needs a read deadline to enforce the pong.

## Justification for the effort level
Medium. Correct keepalive means a `SetReadDeadline` loop that resets on any frame and times out if no pong arrives, coordinated with the existing single-reader goroutine. Getting the deadline semantics right without breaking the "follow" stream is the non-trivial part.
