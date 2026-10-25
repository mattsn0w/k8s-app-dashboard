# SEC-06: workload-url allows open-redirect / javascript: links

- **Type:** Security
- **Summary:** The `workload-url` annotation is rendered directly into an `<a href>` in the table. `isValidWorkloadURL` rejects `http/https`-less and non-`/`-prefixed values, but a relative URL starting with `/` (or an absolute URL if the check is loosened) is not validated against the dashboard's origin, so it can point the browser at an external site or, if validation is weakened, a `javascript:` handler.
- **Severity:** Low
- **Effort:** Low
- **Status:** ✅ **FIXED** (2026-10-25)

## Why this issue should be addressed / ranked
It is a classic link-injection/open-redirect smell. Low because it requires an attacker who can set the deployment annotation (i.e., someone who already has write access to the namespace), so it is not independently exploitable. Ranked low for that reason.

## Justification for the severity level
Low. Exploitation needs prior write access to the workload's annotations; the risk is a phishing-style "open app" link, not data compromise. `rel="noopener noreferrer"` is already present, which neutralizes `window.opener` abuse.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing.

## Justification for the effort level
Low. Tighten `isValidWorkloadURL` to allow only `/`-prefixed same-origin relative paths (or an allow-listed host suffix), and sanitize on render. A few lines plus the existing `url_test.go` cases.

## Resolution (FIXED)

Closed the open-redirect / link-injection at **two** layers (defense-in-depth):

### 1. Backend — `cmd/main.go` `isValidWorkloadURL`
Tightened to accept **only same-origin relative paths**:
- an absolute path starting with a single `/` (e.g. `/apps/echo`, `/apps/vscode/`), or
- a relative subpath ending in `/` with no leading `/` or `//` (e.g. `apps/vscode/`).

It now **rejects**:
- `http(s)://` and any URL containing `://` (absolute / external-site links — the
  open-redirect vector),
- protocol-relative `//host` URLs,
- any non-path scheme (`javascript:`, `data:`, `mailto:`, `file:`, …),
- bare domains, empty strings, whitespace, and control characters.

A deployment whose `workload-url` is anything other than a same-origin relative
path is now **excluded** from the list (the existing filter in `listDeployments`
skips invalid values), so the bad link never even reaches the frontend.

### 2. Frontend — `web/index.html` `safeWorkloadURL()`
Added a render-time guard that mirrors the backend rules. The `<a href>` is only
emitted when `safeWorkloadURL(d.workloadUrl)` returns a safe same-origin path;
anything else (external URL, protocol-relative, `javascript:`/`data:`, bare
root, whitespace, non-string) yields `""` and the link is hidden. This is the
"sanitize on render" half of the issue — even if backend validation were ever
loosened, the SPA will not render a dangerous href. `rel="noopener noreferrer"`
and `target="_blank"` remain as additional hardening.

### Tests
- **`cmd/url_test.go`** — the former "valid absolute URL" `http(s)://` cases now
  expect `false`; added protocol-relative (`//evil`, `//`, `///`) and
  `http(s)://user:pass@host` rejection cases.
- **`web/test.js`** — added 16 `safeWorkloadURL` cases (kept same-origin paths;
  hidden for `http://`, `https://`, `//`, `javascript:`, `data:`, `mailto:`,
  `file:`, `/`, empty, whitespace, null, non-string).

Verified: all 3 demo workloads (`/apps/echo`, `/apps/terminal`,
`/apps/vscode/`) still render their links; the backend filters any
`http(s)://` workload-url out of the list; the Go suite and the 67-case JS suite
pass; deployed to the cluster.
