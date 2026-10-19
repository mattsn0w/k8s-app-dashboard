# SEC-06: workload-url allows open-redirect / javascript: links

- **Type:** Security
- **Summary:** The `workload-url` annotation is rendered directly into an `<a href>` in the table. `isValidWorkloadURL` rejects `http/https`-less and non-`/`-prefixed values, but a relative URL starting with `/` (or an absolute URL if the check is loosened) is not validated against the dashboard's origin, so it can point the browser at an external site or, if validation is weakened, a `javascript:` handler.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
It is a classic link-injection/open-redirect smell. Low because it requires an attacker who can set the deployment annotation (i.e., someone who already has write access to the namespace), so it is not independently exploitable. Ranked low for that reason.

## Justification for the severity level
Low. Exploitation needs prior write access to the workload's annotations; the risk is a phishing-style "open app" link, not data compromise. `rel="noopener noreferrer"` is already present, which neutralizes `window.opener` abuse.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing.

## Justification for the effort level
Low. Tighten `isValidWorkloadURL` to allow only `/`-prefixed same-origin relative paths (or an allow-listed host suffix), and sanitize on render. A few lines plus the existing `url_test.go` cases.
