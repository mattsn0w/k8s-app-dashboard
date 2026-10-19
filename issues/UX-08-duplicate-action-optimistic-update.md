# UX-08: Optimistic UI update can diverge from real state

- **Type:** UX
- **Summary:** After a scale, the handler sets `d.replicas`/`d.running` optimistically and schedules a refresh after 1.5 s. If the refresh is slow or the poll overlaps, the UI can briefly show a state that disagrees with the cluster.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
Optimistic updates feel good but can flicker or lie for a moment. Low because it self-corrects on the next poll; it is a polish issue.

## Justification for the severity level
Low. Transient visual inconsistency, self-healing. No data impact.

## Dependencies
- **Depends on:** none. Pairs with UX-02 (disable while in flight) and UX-07 (surface the real error if the optimistic update is wrong).
- **Enables:** nothing.

## Justification for the effort level
Low. Either drop the optimistic mutation and rely on the (immediate) refresh, or mark the row "updating…" until the confirmed refresh returns. A few lines.
