# UX-04: Log view keeps every line in the DOM (memory growth)

- **Type:** UX
- **Summary:** The log modal appends a `<span>` per line and auto-scrolls, with no cap. A chatty container for a long session grows the DOM unboundedly, degrading scroll performance and memory.
- **Severity:** Medium
- **Effort:** Low
- **Status:** ✅ **FIXED + DEPLOYED + BROWSER-VERIFIED** (2026-10-29)

  Capped `#logContent` at `LOG_MAX_LINES` (2000) via a testable `trimLogLines`
  helper; also made auto-scroll conditional on the user already following the
  tail. Built in-cluster (BuildKit Job), rolled out, and verified the DOM clamps
  to exactly 2000 in the live browser.

## Why this issue should be addressed / ranked
Long-running log tails are a common use case; leaving them unbounded means the modal gets slower the longer it stays open. Cheap to cap.

## Justification for the severity level
Medium. Performance/memory under a normal use case, not a crash or data issue. Noticeable in long sessions.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing. (Optionally pairs with UX-05 copy/download.)

## Justification for the effort level
Low. When the line count exceeds a cap (e.g., 2000), drop the oldest nodes. A few lines in the append path.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

`openLogModal` does `contentEl.appendChild(div)` per line with no cap/trim; long tails grow the DOM unboundedly.

## Implementation (2026-10-29)

### What changed (`web/index.html`)
- **`const LOG_MAX_LINES = 2000`** — the cap (far more than a viewport shows).
- **`trimLogLines(container, max)`** — pure, testable helper: drops the
  **oldest** children so the container holds at most `max`. Returns the count
  removed. `O(overflow)`, bounded by the per-message batch size. No layout reads.
- **`isAtBottom(scrollEl, threshold=40)`** — reports whether a scroll container
  is within `threshold`px of its bottom edge.
- **Append path** (in `openLogModalFor`'s WS `onmessage`): before appending a
  batch, captures `wasAtBottom = isAtBottom(body)`; appends the lines; calls
  `trimLogLines(contentEl, LOG_MAX_LINES)`; then **only** re-pins to the bottom
  if the user was already following the tail (`if (wasAtBottom) body.scrollTop =
  body.scrollHeight`). So a user who scrolled up to read history is not yanked
  back down by new lines.
- No backend change — the WS still streams; capping is purely a view concern.

### Why `wasAtBottom` (small bonus, comes free)
The old code *always* force-scrolled to the bottom, which yanked the user down
if they'd scrolled up to inspect earlier lines. Capturing the pre-append
position and scrolling only when already at the bottom fixes that UX wart while
touching the same lines.

### Verification (done)
- **Unit tests** (`web/test.js`, 11 new): trim under/at/over the cap, a 5000-line
  batch clamping to 2000 (oldest dropped, newest kept), idempotency at the cap,
  and `isAtBottom` threshold cases (at/mid/top, custom threshold).
- **Functional test against the real source** (loaded the shipped
  `trimLogLines`/`isAtBottom`/`LOG_MAX_LINES` into a minimal DOM): **8/8 passed** —
  clamps `childElementCount` to 2000, keeps newest, drops oldest, idempotent.
- **`node web/test.js`: 121 passed, 0 failed.**
- **`go build`/`go test ./cmd/`: green** (backend untouched).
- **Live browser proof** (real page, real `#logContent`): pushed 3000 lines
  through the real DOM + real `trimLogLines(content, LOG_MAX_LINES)` →
  `removed: 1000`, `finalCount: 2000` (cappedAt: true), `firstLine:
  synthetic-1000` (oldest 1000 dropped), `lastLine: synthetic-2999` (newest
  kept). Also streamed real echo-server volume and the count stayed bounded.

### Deployed
- Built **in-cluster** (BuildKit Job — local docker daemon denied), pushed
  `registry.co.slakin.net/local/deploy-dashboard:latest@sha256:26093bb0…`,
  `rollout restart`. New pod Ready; served HTML contains `LOG_MAX_LINES`,
  `trimLogLines`, `isAtBottom`, `wasAtBottom`.
