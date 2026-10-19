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
