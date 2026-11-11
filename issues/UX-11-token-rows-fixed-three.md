# UX-11: Access-token editor is fixed to exactly 3 rows

- **Type:** UX
- **Summary:** The tokens modal renders exactly three hardcoded key/value rows. Users who need 1 or 8 tokens get an awkward form; there is no add/remove row and no validation of the key name.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
A fixed 3-row grid is arbitrary. Users with few tokens see empty boxes; users with many have nowhere to put them. Low because it still works for the common 1–3 case.

## Justification for the severity level
Low. Usability of a secondary feature. The data model (key/value map) is fine; only the form is rigid.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing. (Should keep in sync with how SEC-02 stores the map.)

## Justification for the effort level
Low. Make rows dynamic (add/remove buttons, iterate the map) and validate key format. A modest amount of DOM logic.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

The tokens modal still renders exactly 3 hardcoded rows (`tokenKey0/1/2`); no add/remove, no key validation.

## Resolution — 2026-11-10 (UX-11)

**Status: ✅ FIXED + deployed + browser-verified**

- Replaced the fixed 3-row token grid (`tokenKey0/1/2`) with a **dynamic** list:
  `Add token` button (official `plus-16` Octicon) + a per-row remove button. Opening the
  modal now renders one row per stored token (or a single empty row if none).
- `collectTokens()` validates on save: keys must match `^[A-Za-z_][A-Za-z0-9_]*$`, empty
  rows are ignored, and blank values / duplicate names produce a clear error toast instead
  of silently saving a malformed map.
- Verified live: add/remove adjust row counts; invalid (`1BAD`) and duplicate keys rejected.
- 10 new unit tests (validation matrix + source wiring).
