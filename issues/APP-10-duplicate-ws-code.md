# APP-10: Duplicated WebSocket frame codec (bufio + reader variants)

- **Type:** Anti-pattern
- **Summary:** Two near-identical frame readers/writers exist: `wsWriteFrame`/`wsReadFrame` (operating on `*bufio.ReadWriter`) and `wsWriteFrameConn`/`wsReadFrameReader` (operating on `io.Reader`/`net.Conn`). The bufio variants are effectively dead (only the reader variants are used at runtime; `bufio.Scanner{}` is even referenced just to keep the import alive), so the code carries two copies of a security-sensitive codec.
- **Severity:** Low
- **Effort:** Medium
- **Status:** ✅ **FIXED** (2026-10-27)

## Why this issue should be addressed / ranked
The frame codec is security-sensitive (we already fixed a real length-parsing bug here). Two copies double the surface for a future bug and for audit. Keeping dead variants around invites "which one do I test?" confusion.

## Justification for the severity level
Low. No live bug (the active path is correct and covered by tests), purely a maintainability/audit-surface issue.

## Dependencies
- **Depends on:** none.
- **Enables:** simpler `ws_test.go` and fewer code paths to review for APP-07/08.

## Justification for the effort level
Medium. Safe deletion means proving the bufio variants are truly unreachable, removing the dead functions and the `bufio` import, and confirming the test suite still passes on the single code path. Low risk but touches tested code, so Medium.

## Resolution (FIXED)

The duplicated frame codec is gone. The WebSocket I/O now uses a **single**
writer and reader:

- **Deleted** `wsWriteFrame(rw *bufio.ReadWriter, conn net.Conn, ...)` and
  `wsReadFrame(rw *bufio.ReadWriter)` — the bufio variants.
- **Replaced** the one runtime call to the bufio writer (the ping→pong reply in
  `handleLogsWS`) with the conn writer, so **all** writes on a connection now go
  through `wsWriteFrameConn`. This also removes the latent hazard of two
  different writers (bufio + conn) on the same socket.
- **Removed** the `_ = bufio.Scanner{} // keep import` hack.
- `bufio` remains imported, but only legitimately: `stripCORSWriter.Hijack()`
  returns `*bufio.ReadWriter` (required by the `http.Hijacker` interface) and
  `handleLogsWS` drains the buffered bytes from the Hijacked reader.

The surviving codec is `wsWriteFrameConn` (writer) + `wsReadFrameReader`
(reader) + `wsCombinedReader` (drain-then-conn reader).

### New regression tests (`cmd/ws_test.go`)
- `TestWSWriteRead_RoundTrip_Pong` — the pong path (opcode 0xA) that previously
  only used the deleted bufio writer now round-trips through the conn writer.
- `TestWSWriteRead_RoundTrip_Pong_AllLengthTiers` — pong frames across all
  length tiers (0,1,125,126,127,65535,65536,65537) via the single codec.
- `TestWSWriteFrameConn_Pong_HeaderLayout` — pins the exact wire bytes
  (`0x8A` + length, unmasked) for the pong header the single writer produces.
- `TestWSCoade_SingleCodec_NoBufioVariant` — confirms the conn-writer/reader
  pair is the active, matched codec.

### Verification
- `go build ./cmd/` — clean.
- `go vet ./cmd/` — clean.
- `go test ./cmd/` — **66 passed, 0 failed** (all pre-existing + 4 new tests,
  8 sub-cases for the length-tier table).
- `node web/test.js` — **67 passed, 0 failed** (frontend unaffected).
