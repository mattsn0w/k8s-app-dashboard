# UX-14: Merged Status column hides per-container detail

- **Type:** UX
- **Summary:** Containers, Replicas, and Status were merged into one "Status" column showing "N containers running." When a multi-container deployment has one bad container, the merged badge doesn't make it obvious which container is unhealthy.
- **Severity:** Medium
- **Effort:** Medium

## Why this issue should be addressed / ranked
The simplification (intentional, to reduce clutter) traded away per-container signal. For a control panel, "which container is down" is a primary question, and the merged view can mask it behind a single green/red.

## Justification for the severity level
Medium. Real diagnostic information is less visible for exactly the cases (unhealthy multi-container) where the user most needs it. Balance with the deliberate minimalism that introduced it.

## Dependencies
- **Depends on:** none. Reuses the pods data already fetched.
- **Enables:** nothing. (Optionally pairs with UX-03 — clicking a container in the status cell could jump to its logs.)

## Justification for the effort level
Medium. Add a hover tooltip / popover listing each container's state (or a small per-container dot row) without un-merging the column. Balancing density and clarity is the work.
