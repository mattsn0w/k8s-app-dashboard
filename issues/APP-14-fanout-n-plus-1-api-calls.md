# APP-14: N+1 K8s API calls per deployments list

- **Type:** Anti-pattern
- **Summary:** Serving `GET /api/deployments` lists deployments and pods and then maps pods→deployments, which is fine, but other endpoints (e.g., per-deployment pod lookups on the logs path) can issue a list per request. With the 10 s frontend poll this multiplies API-server load.
- **Severity:** Low
- **Effort:** Medium

## Why this issue should be addressed / ranked
Efficiency/observability. The main list is already a single list-pods call, so the core path is okay; the concern is the per-request list calls elsewhere and the absence of any caching. Under the poll loop this adds up.

## Justification for the severity level
Low. Performance, not correctness or security. The API server is not in danger at current scale; this is forward-looking.

## Dependencies
- **Depends on:** none. Complements APP-05 (rate limiting) and APP-09 (logging) for the same "be a good API citizen" theme.
- **Enables:** a small in-memory cache (TTL a few seconds) that would also smooth the frontend.

## Justification for the effort level
Medium. A correct short-TTL cache (or a watch) that stays consistent enough for a dashboard is more than a one-liner because cache invalidation on the 10 s poll needs care; hence Medium.
