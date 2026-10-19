# APP-13: Missing .gitignore and module hygiene

- **Type:** Anti-pattern
- **Summary:** There is no `.gitignore`; build artifacts (`/tmp/deploy-dashboard-static` is outside the repo, but any in-tree binary), `embed_gen.go` churn, and ad-hoc files risk being committed. `go.mod` has no `go.sum` and no pinned external deps, which is fine for stdlib-only but undocumented.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
Repo-hygiene. Low because nothing secret is obviously at risk, but the absence of a `.gitignore` plus the generated `embed_gen.go` (APP-01) makes accidental commits likely and noisy.

## Justification for the severity level
Low. Pure hygiene; no runtime or security effect.

## Dependencies
- **Depends on:** APP-01 (the embed approach affects what should be ignored).
- **Enables:** nothing.

## Justification for the effort level
Low. Add a `.gitignore` (binaries, temp, editor files) and a short note in the README about the stdlib-only module. Minutes.
