# APP-04: NodePort / port 31143 hard-coded in docs and script

- **Type:** Anti-pattern
- **Summary:** `build-and-deploy.sh` prints `http://<node-ip>:31143/` and other docs reference fixed NodePorts. The actual Envoy LoadBalancer Service ports (30632/30128) and the dashboard Service differ, so the documented access path is stale and misleading.
- **Severity:** Low
- **Effort:** Low
- **Status:** ✅ **FIXED** (2026-10-27)

## Why this issue should be addressed / ranked
It is a correctness-of-documentation problem: following the script's own output leads operators to the wrong port. Low because it does not affect runtime behavior, but it wastes operator time on every deploy.

## Justification for the severity level
Low. No functional impact; purely operational confusion.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing.

## Justification for the effort level
Low. Derive the access URL from the actual Service (read the LoadBalancer IP / ports via `kubectl get svc`) instead of hard-coding, or remove the stale hint. A few lines in the script.

## Resolution (FIXED)

The stale `http://<node-ip>:31143/` hint is gone, and the script now **derives
the real access URL from the live cluster** instead of hard-coding anything:

- `build-and-deploy.sh` no longer prints any fixed NodePort. At the end it finds
  the Envoy Gateway's LoadBalancer Service (`envoy-gateway-system`), reads its
  `EXTERNAL-IP`, pairs it with the Gateway listener hostname, and prints:
  - `https://<gateway-hostname>/` (the real access path), and
  - the raw LB IP with a note that the Host header must be the gateway hostname.
- `README.md` no longer references the old `embed_gen.go` / `EMBEDDED_HTML`
  override or any fixed NodePort; the "Embedding the Frontend" section and
  project tree now describe the `//go:embed` mechanism (see APP-01).

Verified: the script's URL-derivation block prints
`https://apps01.co.slakin.net/` (hostname `apps01.co.slakin.net`, LB IP
`172.16.1.90`) against the live cluster.

Note: the `31143`/`30632` mentions remaining in `plan_v2.md` are inside the
code-review document itself (describing this very finding), not user-facing docs.
