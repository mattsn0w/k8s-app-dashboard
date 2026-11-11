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

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

Delete uses a generic native `confirm("…This cannot be undone.")` with no preview of affected resources (replicas/pods) and no typed confirmation.

## Resolution — 2026-11-10 (UX-16)

**Status: ✅ FIXED + deployed + browser-verified**

- `confirmAction()` gained an optional `requireText` field (type-to-confirm): when set,
  a `Type <name> to confirm:` input appears and the **OK button stays disabled** until the
  user types exactly that text. Type box is auto-focused; focus still never lands on the
  destructive button by default.
- `deleteDeployment()` now passes `requireText: name` and a richer preview detail —
  "This will remove <name>, N replica(s), M currently running."
- `_confirmResolve()` tears down the input listener, re-enables OK, and hides/clears the
  type box on close, so a later plain confirm isn't left disabled.
- Verified live: OK disabled → still disabled on wrong text → enabled on exact name →
  Cancel made no DELETE call → close resets state. Danger styling + preview detail present.
- 12 new unit tests (enable/disable rule + source wiring guards).
