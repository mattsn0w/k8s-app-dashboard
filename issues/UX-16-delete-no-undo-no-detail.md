# UX-16: Delete confirmation gives no detail and no consequence preview

- **Type:** UX
- **Summary:** The delete `confirm()` is a generic "this cannot be undone" with no preview of what will be removed (pods, replicas, services) and no typed-confirmation for an irreversible action.
- **Severity:** Medium
- **Effort:** Low

## Why this issue should be addressed / ranked
Delete is the one action that is truly irreversible. A bare native confirm is the weakest possible guard for it. Showing "will remove deployment X (N replicas, M pods)" and requiring an explicit in-app confirm materially reduces accidental loss.

## Justification for the severity level
Medium. It guards against data/workload loss, but the backend already deletes cleanly and the action is deliberate (a click), so Medium not High.

## Dependencies
- **Depends on:** UX-01 (the confirm-modal component).
- **Enables:** nothing.

## Justification for the effort level
Low. Enhance the (new) confirm modal to show a summary of affected resources and use danger styling; optional type-to-confirm. Small.
