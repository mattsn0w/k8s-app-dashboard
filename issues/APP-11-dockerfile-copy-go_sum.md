# APP-11: Dockerfile copies go.sum that does not exist

- **Type:** Anti-pattern
- **Summary:** `Dockerfile` line 5 runs `COPY go.mod go.sum ./`, but the repo has **no `go.sum`** (verified). The build would fail at that step.
- **Severity:** Low
- **Effort:** Low
- **Status:** ✅ **RESOLVED** (2026-10-27)

## Why this issue should be addressed / ranked
The image build is broken as written. Low severity because the current deployment path does not build via this Dockerfile (the binary is injected by hand), so it is latent — but it will block any real `docker build`.

## Justification for the severity level
Low. Build breakage, not a runtime or security issue, and currently bypassed by the manual deploy flow.

## Dependencies
- **Depends on:** none. If the module stays stdlib-only with no external deps, `go.sum` may be legitimately absent (or empty).
- **Enables:** a working `make build` / image build path.

## Justification for the effort level
Low. Either `COPY go.mod ./` (and generate `go.sum` with `go mod tidy` if/when deps are added), or create the empty `go.sum`. One line.

## Resolution (RESOLVED)

The Dockerfile no longer hard-codes `go.sum`. Line 13 is:

```dockerfile
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
```

- The `go.sum*` **glob** matches zero or more files, so `COPY` succeeds whether
  or not `go.sum` exists. This is the cleanest of the issue's suggested fixes and
  it future-proofs the build: if external dependencies are ever added (creating a
  `go.sum`), the same line works without modification.
- The module is **stdlib-only** (`go.mod` has no `require` directives), so no
  `go.sum` is legitimately needed — exactly the outcome the issue predicted.

**Verified by a real `docker build`** on 2026-10-27: with no `go.sum` in the
repo, `docker build -t .../deploy-dashboard:app11-verify .` completed successfully
(image exported, `naming to ... app11-verify done`). The `COPY go.mod go.sum* ./`
step did not fail. The throwaway image was removed afterward.

Note: the "build is broken / bypassed by manual deploy" framing in the issue is
also outdated — the repo now deploys **via** this Dockerfile
(`build-and-deploy.sh` → `docker build` + `docker push`), and the build works.
