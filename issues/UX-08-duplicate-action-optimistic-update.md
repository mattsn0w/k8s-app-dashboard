# UX-08: Optimistic UI update can diverge from real state

- **Type:** UX
- **Summary:** After a scale, the handler sets `d.replicas`/`d.running` optimistically and schedules a refresh after 1.5 s. If the refresh is slow or the poll overlaps, the UI can briefly show a state that disagrees with the cluster.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
Optimistic updates feel good but can flicker or lie for a moment. Low because it self-corrects on the next poll; it is a polish issue.

## Justification for the severity level
Low. Transient visual inconsistency, self-healing. No data impact.

## Dependencies
- **Depends on:** none. Pairs with UX-02 (disable while in flight) and UX-07 (surface the real error if the optimistic update is wrong).
- **Enables:** nothing.

## Justification for the effort level
Low. Either drop the optimistic mutation and rely on the (immediate) refresh, or mark the row "updating…" until the confirmed refresh returns. A few lines.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

`toggleDeployment` still does the optimistic mutation `d.replicas=targetReplicas; d.running=run; renderDeployments();` then `setTimeout(fetchDeployments,1500)`.

## Resolution — 2026-11-10 (UX-08)

**Status: ✅ FIXED + deployed + browser-verified**

- `toggleDeployment` no longer mutates the cached row (`d.running`/`d.replicas`) to the
  *desired* value before the cluster agrees. Instead it keeps the row **busy** (UX-02/UX-13
  spinner) and calls `waitForDeploymentState(name, run)`, which polls the real deployments
  list until the deployment reports the expected state (or an 8s timeout).
- The table therefore only ever renders **confirmed cluster data** — it can no longer show a
  state that disagrees with reality. On scale failure it surfaces the error and re-fetches the
  true state.
- Verified live (intercepted scale POST, simulated convergence): scale called -> row stays
  real until a fetched value confirms -> converges -> busy clears. Cluster left untouched.
- 6 new unit tests (regression guards for the removed optimistic assignments).
