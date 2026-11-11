# UX-05: No copy or download of the current log view

- **Type:** UX
- **Summary:** There is no way to copy the visible logs to the clipboard or save them to a file; the user must select text manually.
- **Severity:** Medium
- **Effort:** Low

## Why this issue should be addressed / ranked
For a debugging tool, "I need these logs" is a near-certain action, and manual selection of a scrolling `<pre>` is error-prone. Low effort, high perceived value.

## Justification for the severity level
Medium. Convenience that materially improves the core debugging loop. Not a blocker, so Medium.

## Dependencies
- **Depends on:** none. Reuses the same line buffer as UX-04.
- **Enables:** nothing.

## Justification for the effort level
Low. Add "Copy" (`navigator.clipboard.writeText`) and "Download" (Blob + `<a download>`) buttons in the modal footer. A few lines.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

The log modal footer has only a status text + 'Follow: real-time' label; no Copy/Download buttons or handlers.

## Resolution — 2026-11-10 (UX-05)

**Status: ✅ FIXED + deployed + browser-verified**

### Changes
- Log modal footer now has **Copy** and **Download** buttons (official Primer
  `copy-16`/`download-16` Octicons added to the sprite).
- `buildLogText(contentEl)` — pure, testable helper joining the buffered `.log-line`
  children with newlines (reuses the UX-04 buffer).
- `copyLogView()` — `navigator.clipboard.writeText` with a legacy `execCommand`
  fallback for non-secure contexts; shows a success/error toast.
- `downloadLogView()` — `Blob` + `<a download>` + `URL.createObjectURL`, with
  `revokeObjectURL` cleanup; filename = `<deployment>_<iso-timestamp>.txt`.
- Added base `.btn`/`.btn-sm`/`.btn-primary`/`.btn-outline` CSS (these classes were
  already used by the confirm/settings modals but had **no styling** — this fixes those
  buttons too).

### Verification
- 16 new JS unit tests (`buildLogText` join/empty/null/blank-line behavior, filename
  sanitizer, wiring + sprite presence). Full suite: **147 passed**.
- Live browser: seeded 3 lines → Copy put exactly those lines on the clipboard; Download
  produced `echo-server_2026-11-11T03-10-46-431Z.txt` via a Blob + object URL that was
  then revoked; success toasts shown for both.
