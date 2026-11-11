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

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

No inline `<head>` script sets the theme. `<html>` defaults to `data-color-mode="light"`; the saved theme is applied by `setTheme('frappe')` at line 823 in the main script (after DOM parse/paint) → flash for dark-theme users.

## Resolution — 2026-11-10 (UX-09)

**Status: ✅ FIXED + deployed + browser-verified**

- Added an inline, dependency-free script in `<head>` (right after `<title>`, before the
  stylesheet link) that reads `localStorage['devpod-theme']` and sets `data-color-mode`
  synchronously, before CSS paints — eliminating the theme flash.
- `setTheme()` now writes the theme to `localStorage` as a fast cache. The server
  `user-settings` value remains authoritative and is re-applied on load (which also
  refreshes the cache), so a theme set on another device still wins after the fetch.
- Guarded with `try/catch` (private-mode) and only accepts the known `latte`/`frappe` keys.
- Verified: head script runs before the stylesheet and sets `light` for a `latte` cache.
- 6 new unit tests.
