# UX-03: Log modal silently picks the first pod/container

- **Type:** UX
- **Summary:** `showLogs` fetches the deployment's pods, picks the first ready pod (else first pod) and `containers[0]`, and opens the log modal on that. Multi-container or multi-replica deployments have no way for the user to choose which pod or container's logs to view.
- **Severity:** High
- **Effort:** Medium

## Why this issue should be addressed / ranked
For any deployment with sidecars or multiple replicas, the user sees the wrong logs with no indication or control. Logs are a core feature of a "control panel," so silently showing container 0 of pod 0 is a functional gap, not just polish.

## Justification for the severity level
High (for UX). It makes the primary diagnostic feature misleading for non-trivial workloads. The fix is expected to be "obvious" (a pod/container dropdown), which is what raises its priority.

## Dependencies
- **Depends on:** none. The pod list is already fetched by `showLogs`, so the data is available.
- **Enables:** re-streaming when the user changes selection (reuses the existing WS open/close).

## Justification for the effort level
Medium. Add two `<select>` elements (pods, then containers) populated from the fetched list, wire change handlers to close the current WS and open a new one for the chosen pair, and handle the empty/1-replica case gracefully.
