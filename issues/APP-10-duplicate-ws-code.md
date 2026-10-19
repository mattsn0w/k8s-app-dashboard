# APP-10: Duplicated WebSocket frame codec (bufio + reader variants)

- **Type:** Anti-pattern
- **Summary:** Two near-identical frame readers/writers exist: `wsWriteFrame`/`wsReadFrame` (operating on `*bufio.ReadWriter`) and `wsWriteFrameConn`/`wsReadFrameReader` (operating on `io.Reader`/`net.Conn`). The bufio variants are effectively dead (only the reader variants are used at runtime; `bufio.Scanner{}` is even referenced just to keep the import alive), so the code carries two copies of a security-sensitive codec.
- **Severity:** Low
- **Effort:** Medium

## Why this issue should be addressed / ranked
The frame codec is security-sensitive (we already fixed a real length-parsing bug here). Two copies double the surface for a future bug and for audit. Keeping dead variants around invites "which one do I test?" confusion.

## Justification for the severity level
Low. No live bug (the active path is correct and covered by tests), purely a maintainability/audit-surface issue.

## Dependencies
- **Depends on:** none.
- **Enables:** simpler `ws_test.go` and fewer code paths to review for APP-07/08.

## Justification for the effort level
Medium. Safe deletion means proving the bufio variants are truly unreachable, removing the dead functions and the `bufio` import, and confirming the test suite still passes on the single code path. Low risk but touches tested code, so Medium.
