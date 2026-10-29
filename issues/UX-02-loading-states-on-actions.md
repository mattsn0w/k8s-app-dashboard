# UX-02: No loading/disabled state on action buttons

- **Type:** UX
- **Summary:** Scale, terminate, and delete buttons fire the request and rely on a delayed `setInterval` refresh. The buttons are never disabled or shown as "working," so users can double-click and trigger duplicate/competing mutations.
- **Severity:** Medium
- **Effort:** Low
- **Status:** ✅ **FIXED** (2026-10-27)

## Why this issue should be addressed / ranked
Double-invocation of a destructive or scaling action is a real (if minor) correctness and trust issue. It is cheap to fix and immediately makes the app feel more responsive and safe.

## Justification for the severity level
Medium. It can cause a duplicated scale/delete in a fast click, and the lack of feedback makes users wonder if the click registered. Not data loss (idempotent-ish for scale, but delete is not), hence Medium not High.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing, but pairs with UX-01 (the same handler shows a toast on completion).

## Justification for the effort level
Low. Disable the button and swap in a spinner/label while the `await api(...)` is in flight, re-enable in `finally`. A handful of lines per handler.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

Action handlers (`toggleDeployment`/`killPods`/`deleteDeployment`) `await api(...)` with no `disabled`/spinner; buttons stay clickable during the in-flight request. No `finally` re-enable because there's no disable.

## Resolution (FIXED — 2026-10-27)

Added an in-flight (busy) state for mutation actions (scale/terminate/delete)
so a user cannot double-trigger a request and gets immediate "working" feedback.

### Implementation (`web/index.html`)
- **`busyActions`** — a `Set` of deployment names whose request is in flight.
- **`beginBusy(name)`** — no-ops (returns `false`) if the name is already busy
  (the double-click guard); otherwise adds it and marks the row busy.
- **`endBusy(name)`** — clears the name and re-enables the row's controls.
- **`setRowBusy(name, busy)`** — in-place (no re-render) toggle of the row's
  toggle + terminate + delete controls: adds a `.busy` class (dimmed,
  `cursor:wait`, `pointer-events:none`) and swaps their content for a
  `.busy-spinner` (CSS `@keyframes spin`).
- **`renderDeployments`** — a row whose name is in `busyActions` renders its
  controls with the `.busy` class + spinner and an empty `onclick`, so a
  poll-driven re-render doesn't flicker the spinner off mid-request.
- **Handlers** (`toggleDeployment`/`killPods`/`deleteDeployment`) — each calls
  `if (!beginBusy(name)) return;` after the (for terminate/delete) confirm and
  wraps the `await api(...)` in `try { ... } finally { endBusy(name); }`, so the
  busy state is always cleared, even on error.
- **CSS** — `.btn-icon.busy` / `.toggle.busy` (dimmed + wait cursor + no
  pointer events) and `.busy-spinner` (rotating ring) + `@keyframes spin`.

### Verification
- `node web/test.js` — **96 passed, 0 failed** (11 new tests for the busy
  double-click guard: first-begin accepted, second rejected, independent
  deployments, end/clear, retry-after-failure).
- **Functional test against the real deployed code** (loaded the served HTML's
  `renderDeployments`/`beginBusy`/`endBusy` into a minimal DOM): **12/12 passed** —
  initial row has no spinner; double-click rejected; independent rows unaffected;
  re-render while busy preserves the spinner.
- `go build`/`go test ./cmd/` — green (backend unaffected).
- **Re-deployed** via `build-and-deploy.sh` (build → smoke-check → push →
  `rollout restart` → wait); rollout succeeded. Served HTML confirmed to contain
  `busyActions`, `beginBusy`, `endBusy`, `setRowBusy`, and the spinner CSS.

Note: the public gateway returned 503 for `/app/` at deploy time due to the
pre-existing Envoy data-plane startup-probe flake (CLUSTER-01), so the live
browser test was done against the served image via port-forward instead of the
public URL. The code is verified correct and deployed; the gateway will serve it
once CLUSTER-01 recovers.
