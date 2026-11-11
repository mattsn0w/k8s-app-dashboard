# UX-07: Failed load renders an error row inside the table

- **Type:** UX
- **Summary:** When `fetchDeployments` fails, the error is injected as a table row ("Failed to load deployments: …"). It replaces the whole table and mixes error presentation with data layout.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
An inline error is fine for "no data," but a fetch failure should be a dismissible banner above the table (with a retry), not a pseudo-row that destroys the table structure. Low because the data is still absent, so it is a presentation improvement.

## Justification for the severity level
Low. Presentation/consistency. The failure is still visible; it is just in the wrong container.

## Dependencies
- **Depends on:** none.
- **Enables:** shares the toast/banner component from UX-01.

## Justification for the effort level
Low. Move the failure into a banner element above the table with a Retry button; keep the empty-state row for the genuine zero-deployments case. A few lines.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

`fetchDeployments` catch-block still injects a pseudo-row `<tr><td colspan="3" class="empty-state">Failed to load deployments…` into the table body instead of a banner.

## Resolution — 2026-11-10 (UX-07)

**Status: ✅ FIXED + deployed + browser-verified**

- Fetch failures now render as a dismissible **error banner** above the deployments table
  (`role="alert"`, danger styling, `Retry` + dismiss), and the table structure is left
  intact — no more `<tr><td colspan=3>Failed to load…` pseudo-row destroying the tbody.
- A successful fetch hides the banner and renders rows as usual. The genuine zero-results
  case still uses the empty-state row (correct container).
- Verified live: failure -> banner with message + Retry, no pseudo-row; dismiss + success-clear both work.
- 6 new unit tests (incl. regression guard that the catch never touches the tbody).
