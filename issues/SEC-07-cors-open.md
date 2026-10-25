# SEC-07: No CORS restriction on API endpoints

- **Type:** Security
- **Summary:** The handlers set no `Access-Control-Allow-Origin` and rely on the browser default (same-origin only). This is currently *safe*, but there is no explicit policy, and a future "fix" that naively adds `*` (or reflects the Origin) would open the JSON API to cross-origin abuse.
- **Severity:** Low
- **Effort:** Low
- **Status:** ✅ **FIXED** (2026-10-25)

## Why this issue should be addressed / ranked
Defense-in-depth / documentation. There is no live bug today (no header is set, so cross-origin fetches are blocked by the browser). The issue is that nothing pins the intended same-origin behavior, so it is easy to regress. Included to make the intended policy explicit and tested.

## Justification for the severity level
Low. No current exploitable path; the risk is a latent regression. Flagging it so the team does not "helpfully" add a permissive CORS header later.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing; purely a guardrail. If the app is ever split behind a different front-end origin, this becomes Medium and needs a real origin allow-list.

## Justification for the effort level
Low. Either leave headers unset (and add a comment/test asserting no `*`), or set an explicit same-origin policy. Both are minutes.

## Resolution (FIXED)

Pinned the same-origin policy centrally so it can't regress:

- **`cmd/main.go`** — added a `sameOriginOnly` middleware that wraps the entire
  handler chain (wired into `main()` around `basePathHandler`). It uses a
  `stripCORSWriter` that removes `Access-Control-Allow-Origin`,
  `Access-Control-Allow-Credentials`, and any `Origin` value from `Vary` the
  moment the response is written — i.e. *after* a handler has set its headers.
  So even a future handler that naively sets `Access-Control-Allow-Origin: *`
  (or reflects a caller's Origin) is neutralized. The browser's default
  same-origin policy then blocks cross-origin reads.
- **`cmd/cors_test.go`** — added `TestNoPermissiveCORS`, which sends
  cross-origin requests (with `Origin: https://evil.example`) to a handler that
  *tries* to set a permissive CORS header and asserts the response contains no
  `Access-Control-Allow-Origin`, no `Access-Control-Allow-Credentials`, and no
  `Origin` in `Vary`.

Verified: cross-origin request to `/api/deployments` returns `200` with no CORS
headers; the SPA and API still work normally.
