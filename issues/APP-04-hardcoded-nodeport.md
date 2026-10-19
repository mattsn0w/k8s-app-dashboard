# APP-04: NodePort / port 31143 hard-coded in docs and script

- **Type:** Anti-pattern
- **Summary:** `build-and-deploy.sh` prints `http://<node-ip>:31143/` and other docs reference fixed NodePorts. The actual Envoy LoadBalancer Service ports (30632/30128) and the dashboard Service differ, so the documented access path is stale and misleading.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
It is a correctness-of-documentation problem: following the script's own output leads operators to the wrong port. Low because it does not affect runtime behavior, but it wastes operator time on every deploy.

## Justification for the severity level
Low. No functional impact; purely operational confusion.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing.

## Justification for the effort level
Low. Derive the access URL from the actual Service (read the LoadBalancer IP / ports via `kubectl get svc`) instead of hard-coding, or remove the stale hint. A few lines in the script.
