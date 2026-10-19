# UX-02: No loading/disabled state on action buttons

- **Type:** UX
- **Summary:** Scale, terminate, and delete buttons fire the request and rely on a delayed `setInterval` refresh. The buttons are never disabled or shown as "working," so users can double-click and trigger duplicate/competing mutations.
- **Severity:** Medium
- **Effort:** Low

## Why this issue should be addressed / ranked
Double-invocation of a destructive or scaling action is a real (if minor) correctness and trust issue. It is cheap to fix and immediately makes the app feel more responsive and safe.

## Justification for the severity level
Medium. It can cause a duplicated scale/delete in a fast click, and the lack of feedback makes users wonder if the click registered. Not data loss (idempotent-ish for scale, but delete is not), hence Medium not High.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing, but pairs with UX-01 (the same handler shows a toast on completion).

## Justification for the effort level
Low. Disable the button and swap in a spinner/label while the `await api(...)` is in flight, re-enable in `finally`. A handful of lines per handler.
