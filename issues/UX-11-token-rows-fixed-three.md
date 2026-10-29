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
