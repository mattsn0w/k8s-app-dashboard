# UX-06: 10 s poll continues when the tab is hidden

- **Type:** UX
- **Summary:** `setInterval(..., 10000)` keeps polling `fetchDeployments` even when the tab is hidden or the app is idle, wasting K8s API calls (see APP-14) and battery/network.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
A hidden tab polling every 10 s is pure waste that also contributes to API-server load. Easy to gate.

## Justification for the severity level
Low. Efficiency/hygiene, not correctness or a user-visible defect while the tab is in front.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing. Complements APP-05/APP-14 (reduces load).

## Justification for the effort level
Low. Pause the interval on `document.visibilitychange` hidden, resume + immediate refresh on visible. A few lines.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

`setInterval(refreshAll, 10000)` at the end of the script; no `document.visibilitychange` handler to pause when the tab is hidden.
