# UX-17: Single theme-toggle semantics are unclear

- **Type:** UX
- **Summary:** There is one cycle/switch button in the user menu ("Switch to Dark"/"Switch to Light"). Users coming from the old 3-theme UI (Latte/Frappe/Mocha) may not realize only light/dark are offered, and the single control's state is not visually obvious at a glance.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
The consolidation to light/dark was deliberate (Primer maps to `data-color-mode`), but the affordance should make the current state obvious (sun/moon icon that reflects the active theme) and optionally hint that more themes are unavailable. Low because the control works; it is discoverability.

## Justification for the severity level
Low. The toggle functions correctly; the issue is making its state and scope legible.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing.

## Justification for the effort level
Low. Swap the icon to reflect the *current* theme (moon when light, sun when dark) and add a `title`. A few lines.
