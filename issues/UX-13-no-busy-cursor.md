# UX-13: No cursor/wait affordance during long operations

- **Type:** UX
- **Summary:** During network calls (scale, delete, log connect) there is no global busy affordance beyond the (currently absent) per-button state. Users get no cue that the app is working.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
Perceived responsiveness. Low because it is covered largely by UX-02 (disable the specific button), so this is a lighter-touch complement (e.g., a subtle top progress bar).

## Justification for the severity level
Low. Perceptual. The specific-button fix (UX-02) removes most of the need.

## Dependencies
- **Depends on:** UX-02 (build on the in-flight state rather than duplicating it).
- **Enables:** nothing.

## Justification for the effort level
Low. Reuse the in-flight flag from UX-02 to toggle a `wait` cursor or a thin top progress bar. Trivial.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

No `cursor:wait` / progress bar / global busy affordance anywhere; relies entirely on the (absent) per-button state from UX-02.

## Resolution — 2026-11-10 (UX-13)

**Status: ✅ FIXED + deployed + browser-verified**

- Added a global busy affordance driven by the UX-02 `busyActions` set (single source of
  truth, so it never drifts from the per-button state): a thin indeterminate **top progress
  bar** plus a **`cursor: wait`** on the whole page while any action is in flight.
- `updateGlobalBusy()` runs on every `beginBusy`/`endBusy`; toggles the bar's `.active`,
  `aria-hidden`, and `body.busy`.
- 6 new unit tests. Verified live: beginBusy -> bar active + wait cursor + aria-hidden=false;
  endBusy -> all cleared.
