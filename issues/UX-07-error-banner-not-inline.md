# UX-07: Failed load renders an error row inside the table

- **Type:** UX
- **Summary:** When `fetchDeployments` fails, the error is injected as a table row ("Failed to load deployments: …"). It replaces the whole table and mixes error presentation with data layout.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
An inline error is fine for "no data," but a fetch failure should be a dismissible banner above the table (with a retry), not a pseudo-row that destroys the table structure. Low because the data is still absent, so it is a presentation improvement.

## Justification for the severity level
Low. Presentation/consistency. The failure is still visible; it is just in the wrong container.

## Dependencies
- **Depends on:** none.
- **Enables:** shares the toast/banner component from UX-01.

## Justification for the effort level
Low. Move the failure into a banner element above the table with a Retry button; keep the empty-state row for the genuine zero-deployments case. A few lines.
