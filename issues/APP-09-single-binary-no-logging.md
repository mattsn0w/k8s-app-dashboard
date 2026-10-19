# APP-09: No structured logging / request logging

- **Type:** Anti-pattern
- **Summary:** Beyond a couple of `log.Printf`s, there is no request logging, no request IDs, and no log level control. Operational diagnosis of "who scaled what / who deleted what" is impossible after the fact.
- **Severity:** Low
- **Effort:** Medium

## Why this issue should be addressed / ranked
For an app whose whole job is to mutate cluster state (scale/delete/terminate), having no audit trail of who did what is an operational gap. Also makes SEC-05 (error logging) have nowhere to live.

## Justification for the severity level
Low. No direct security or correctness impact; it is observability. But for a control-plane app it is a meaningful gap.

## Dependencies
- **Depends on:** none. Once SEC-01 (auth) exists, the structured log should include the identity.
- **Enables:** a clean home for the full (unsanitized) error text in SEC-05.

## Justification for the effort level
Medium. Worth doing properly (structured fields, method/path/duration/identity, a level toggle) rather than sprinkling `log.Printf`, which is why it is Medium not Low.
