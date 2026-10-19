# SEC-08: WebSocket frame size cap is 1 MiB (potential memory pressure)

- **Type:** Security
- **Summary:** `wsReadFrameReader`/the reader reject frames above 1 MiB (`frame too large`). The cap exists and is good, but a client that streams many near-cap frames (or a log burst) can still drive significant transient memory and CPU per connection, and there is no per-connection frame *rate* limit.
- **Severity:** Low
- **Effort:** Medium

## Why this issue should be addressed / ranked
Resource-exhaustion hardening on the long-lived WebSocket. Low because the 1 MiB cap already prevents the catastrophic single-frame allocation, and connections are still bounded by SEC-01's reach. Ranked low but worth noting for a production posture.

## Justification for the severity level
Low. The existing cap makes a true DoS hard; the residual risk is sustained bandwidth/CPU from many open streaming connections, which is a load problem more than a security one. It is listed here because an unauthenticated streaming endpoint (SEC-01) is what makes it reachable.

## Dependencies
- **Depends on:** SEC-01 (auth) is the primary control; this is secondary.
- **Enables:** nothing.

## Justification for the effort level
Medium. A token-bucket rate limit per connection plus a configurable smaller cap touches the WS reader loop and the K8s log pump, and needs care not to drop legitimate log bursts — hence Medium rather than Low.
