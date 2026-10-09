# Devpod Test Plan

## Implemented Tests

### Go Unit Tests (`cmd/`)

| File | Tests | Coverage |
|------|-------|----------|
| `ws_test.go` | 36 | WebSocket frame codec (read/write/round-trip), combined reader, handshake |
| `url_test.go` | 16 | `isValidWorkloadURL` validation (valid/invalid/edge cases) |
| `main_test.go` | 14 | K8s client methods (listDeployments, scaleDeployment, deletePods, deleteDeployment, getUserSettings, upsertUserSettings), HTTP handlers (handleDeployments, handleHealthz, handleIndex) |

**Total: 53 Go tests, all passing.**

### JavaScript Unit Tests (`web/`)

| File | Tests | Coverage |
|------|-------|----------|
| `test.js` | 28 | Theme switching, API path construction (subpath support), WebSocket URL construction, log line processing, deployment status/target replicas |

**Total: 28 JS tests, all passing.**

### How to Run

```bash
# In a developer sandbox or CI:
make test        # Go unit tests
make test-js     # JavaScript unit tests
make vet         # go vet
make build       # static binary build

# Or directly:
go test -v -count=1 ./cmd/
node web/test.js
```

### CI (GitHub Actions)

The `.github/workflows/ci.yml` workflow runs on every push/PR to main:
1. `go vet ./cmd/`
2. `go test -v -count=1 -race ./cmd/`
3. `CGO_ENABLED=0 go build` (verify static binary compiles)
4. `node web/test.js`

### What's NOT Tested (and why)

- **WebSocket end-to-end**: Requires a live K8s cluster with a running pod. The frame codec is thoroughly unit-tested; the end-to-end path (HTTP upgrade → K8s log stream → WS frames) is verified manually via the browser.
- **K8s API integration**: Requires a real cluster. The client methods are tested with `httptest` mocks that verify HTTP method, path, headers, and response decoding.
- **Frontend DOM rendering**: The JS is tightly coupled to the DOM. Pure logic functions (theme, path construction, log parsing) are tested; DOM manipulation is verified via browser screenshots.

### Test Invariants (Must Hold)

1. **WebSocket frame codec**: All 7 length encodings (0-125, 126, 127, 16-bit, 64-bit) must round-trip correctly. Masked frames must decode to the original payload.
2. **URL validation**: Only `http://`, `https://`, or `/`-prefixed URLs are accepted. No whitespace, control chars, or other schemes.
3. **Subpath support**: API calls and WebSocket URLs must work under any subpath (e.g., `/apps/dashboard/`).
4. **Theme switching**: Latte ↔ Frappe must toggle correctly, with `data-color-mode` set to `light`/`dark`.
5. **Deployment state**: Scale to 0 must preserve the last replica count in an annotation. Scale from 0 must restore to 1.
