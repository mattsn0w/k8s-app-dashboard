# UX-18: Table rows hide detail behind no tooltip / no row-expand

- **Type:** UX
- **Summary:** Each row shows name, status, and actions. There is no hover tooltip or expandable row to surface extra context (image, restart count, age, pod IPs, annotations like workload-url) without opening the log modal.
- **Severity:** Low
- **Effort:** Medium

## Why this issue should be addressed / ranked
The simplified 3-column layout is clean but pushes useful context out of reach. A lightweight tooltip or expand reveals detail on demand without re-adding columns. Ranked low because it is an enhancement over a working layout.

## Justification for the severity level
Low. Enhancement, not a defect. The primary data is visible; this is about reachability of secondary data.

## Dependencies
- **Depends on:** the extra fields being available from the API (image/age may need a small backend addition).
- **Enables:** nothing. Pairs with UX-14 (per-container detail) as "detail on demand."

## Justification for the effort level
Medium. A tooltip is cheap; a proper expandable row (fetching pod/replica-set detail on demand, rendering image/restarts/age) involves both API surface and DOM, hence Medium.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

Deployment rows are 3 columns (Name/Status/Actions); no hover tooltip or expandable row for image/restarts/age/pod IPs/annotations.
