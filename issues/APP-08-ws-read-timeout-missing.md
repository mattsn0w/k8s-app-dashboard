# APP-08: No read deadline on the WebSocket

- **Type:** Anti-pattern
- **Summary:** The WS reader blocks on `wsReadFrameReader` with no `SetReadDeadline`. A client that stops sending (but keeps the socket open) parks the reader goroutine forever.
- **Severity:** Low
- **Effort:** Medium

## Why this issue should be addressed / ranked
Same family as APP-07: unbounded goroutine lifetime on half-open connections. Without a deadline the only way out is a write error or the client sending a close frame.

## Justification for the severity level
Low. Resource leak, not data or crash. Bounded in practice by connection count (SEC-01) but still a latent leak.

## Dependencies
- **Depends on:** none. Naturally implemented together with APP-07 (the ping/pong keepalive that the deadline enforces).
- **Enables:** nothing.

## Justification for the effort level
Medium. Must integrate with the existing reader goroutine and the "follow" semantics (a quiet log is legitimate, so the deadline has to be driven by pings/pongs, not by log silence). That coordination is the effort.
