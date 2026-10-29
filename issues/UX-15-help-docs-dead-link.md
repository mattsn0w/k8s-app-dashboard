# UX-15: Help/Docs menu link points to a placeholder

- **Type:** UX
- **Summary:** The user menu's "Help/Docs" link points to `http://example.co.slakin.net` (a placeholder). It dead-ends and looks unfinished.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
A placeholder link in shipped UI is a visible "not finished" signal. Trivial to correct once a real docs URL exists.

## Justification for the severity level
Low. No functional impact; purely a placeholder to replace.

## Dependencies
- **Depends on:** the existence of a real docs URL (external).
- **Enables:** nothing.

## Justification for the effort level
Low. Replace the href with the real docs URL (or remove the item until it exists). One line.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

The user menu 'Help/Docs' link still points to the placeholder `http://example.co.slakin.net` (line 177).
