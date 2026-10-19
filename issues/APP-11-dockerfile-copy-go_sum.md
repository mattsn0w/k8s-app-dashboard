# APP-11: Dockerfile copies go.sum that does not exist

- **Type:** Anti-pattern
- **Summary:** `Dockerfile` line 5 runs `COPY go.mod go.sum ./`, but the repo has **no `go.sum`** (verified). The build would fail at that step.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
The image build is broken as written. Low severity because the current deployment path does not build via this Dockerfile (the binary is injected by hand), so it is latent — but it will block any real `docker build`.

## Justification for the severity level
Low. Build breakage, not a runtime or security issue, and currently bypassed by the manual deploy flow.

## Dependencies
- **Depends on:** none. If the module stays stdlib-only with no external deps, `go.sum` may be legitimately absent (or empty).
- **Enables:** a working `make build` / image build path.

## Justification for the effort level
Low. Either `COPY go.mod ./` (and generate `go.sum` with `go mod tidy` if/when deps are added), or create the empty `go.sum`. One line.
