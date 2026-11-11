# UX-10: Weak accessibility (ARIA/labels) on interactive controls

- **Type:** UX
- **Summary:** Icon-only buttons (logs, terminate, delete, theme, user menu), the toggle switch, and the modals lack accessible names (`aria-label`), roles, and focus management. Screen-reader and keyboard users get a degraded experience.
- **Severity:** Medium
- **Effort:** Medium

## Why this issue should be addressed / ranked
Accessibility is a real usability requirement, not optional polish. Icon-only controls with no `aria-label` are invisible to assistive tech, and modals that don't trap/focus manage are hard to use without a mouse.

## Justification for the severity level
Medium. Affects a subset of users and keyboard workflows; not a blocker for the majority, but a genuine gap for a control-panel tool.

## Dependencies
- **Depends on:** none.
- **Enables:** better keyboard flow (Escape-to-close already exists; focus trapping would build on it).

## Justification for the effort level
Medium. Adding `aria-label`s is quick, but proper modal focus trapping, `role`/`aria-expanded` on the menu, and a focusable toggle switch with `role="switch"` is a broader pass — hence Medium.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

Only 1 `aria-label` in the whole file. Icon-only buttons (logs/terminate/delete, user menu, theme/view-mode/SSH/tokens, modal closes) have no accessible name. Tabs have `role="tab"` but the rest of the controls lack roles/labels/focus management.

## Resolution — 2026-11-10 (UX-10)

**Status: ✅ FIXED + deployed + browser-verified**

### Accessible names & roles
- The deployment/job **toggle switch** is now a real `role="switch"` with `aria-checked`,
  `tabindex=0`, a contextual `aria-label` ("Start/Stop <name>"), and `onkeydown` handling
  (Space/Enter activate via `toggleKey`). Previously an unlabeled `<div>`.
- All **icon-only action buttons** now carry contextual `aria-label`s ("View logs for X",
  "Terminate pods for X", "Delete deployment X", "View output for job X"); their octicons
  are marked `aria-hidden`.
- **Every** modal-close button got an `aria-label` (log window, SSH key dialog, tokens
  dialog — the confirm/help ones already had them).
- Log/SSH/tokens overlays upgraded to `role="dialog" aria-modal="true"` with
  `aria-labelledby`/`aria-label`.
- The user-menu button exposes `aria-haspopup` + `aria-expanded` (kept in sync by
  `toggleUserMenu`/`closeUserMenu`); the menu has `role="menu"`.

### Focus management
- A single global **focus trap** (MutationObserver on every `.modal-overlay`): on open it
  focuses the first control (saving the previous focus), traps Tab to cycle within the
  dialog, and restores focus to the triggering element on close. Works for all modals
  regardless of which function opened them.

### Verification
- 22 new unit tests (switch role/keyboard, button labels, modal-close labels, dialog roles,
  menu expanded state, focus-trap presence/behavior). Full suite: **251 passed**.
- Live browser: toggle reports role=switch/aria-checked/label/tabindex; buttons labelled;
  menu aria-expanded toggles; opening Help moves focus into the dialog; Tab wraps last->first;
  closing restores focus. aria-modal dialog on logModal confirmed.

Inventory before -> after: aria-label 13 -> 24, role 10 -> 16, aria-modal 1 -> 5.
