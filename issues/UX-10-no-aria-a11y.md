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
