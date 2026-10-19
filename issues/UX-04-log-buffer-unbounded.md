# UX-04: Log view keeps every line in the DOM (memory growth)

- **Type:** UX
- **Summary:** The log modal appends a `<span>` per line and auto-scrolls, with no cap. A chatty container for a long session grows the DOM unboundedly, degrading scroll performance and memory.
- **Severity:** Medium
- **Effort:** Low

## Why this issue should be addressed / ranked
Long-running log tails are a common use case; leaving them unbounded means the modal gets slower the longer it stays open. Cheap to cap.

## Justification for the severity level
Medium. Performance/memory under a normal use case, not a crash or data issue. Noticeable in long sessions.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing. (Optionally pairs with UX-05 copy/download.)

## Justification for the effort level
Low. When the line count exceeds a cap (e.g., 2000), drop the oldest nodes. A few lines in the append path.
