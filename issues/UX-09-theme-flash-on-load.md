# UX-09: Theme applies after first paint (flash of wrong theme)

- **Type:** UX
- **Summary:** The saved theme is read from `localStorage` and applied in the main script, which runs after the DOM parses. Users see a brief flash of the default theme before the saved one is applied, especially on a slow CDN.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
A theme flash on every load is a small but frequent jolt. Standard fix is a tiny inline `<head>` script that sets the attribute before CSS applies.

## Justification for the severity level
Low. Cosmetic, per-load, self-resolving. High-frequency but trivial.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing.

## Justification for the effort level
Low. Add a 3-line inline script in `<head>` that sets `data-theme`/`data-color-mode` from `localStorage` before the body renders.
