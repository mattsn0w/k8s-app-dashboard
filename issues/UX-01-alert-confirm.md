# UX-01: alert()/confirm() for all feedback and destructive confirmations

- **Type:** UX
- **Summary:** All user feedback is `alert()` (7 call sites: scale, terminate, delete, fetch-pods errors, save-settings errors) and destructive confirmations are `confirm()` (2 call sites: terminate, delete). These are blocking, unstyled, and break the app's visual identity.
- **Severity:** High
- **Effort:** Medium

## Why this issue should be addressed / ranked
It is the single most-visible polish gap: every error and every destructive action pops a browser dialog that looks nothing like the Primer UI. It is ranked high among UX items because it touches the most-used paths (all actions) and directly undermines trust (a native confirm for "delete deployment" looks amateurish and is easily mis-clicked).

## Justification for the severity level
High (for UX). Not a correctness bug, but it is on every interaction and it makes the product feel unfinished. A non-blocking toast for errors and a proper in-app confirm modal (with typed or explicit confirm for destructive ops) is the expected standard.

## Dependencies
- **Depends on:** none.
- **Enables:** a reusable toast component (used by many other UX items) and a confirm-modal component (used by UX-08 loading states, which disable during the action).

## Justification for the effort level
Medium. Build a small toast (queue, auto-dismiss, success/error variants) and a confirm modal (title, body, cancel/confirm, danger styling), then replace the 9 call sites. The component work is the bulk; the call-site swaps are trivial.
