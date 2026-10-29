# UX-01: alert()/confirm() for all feedback and destructive confirmations

- **Type:** UX
- **Summary:** All user feedback is `alert()` (7 call sites: scale, terminate, delete, fetch-pods errors, save-settings errors) and destructive confirmations are `confirm()` (2 call sites: terminate, delete). These are blocking, unstyled, and break the app's visual identity.
- **Severity:** High
- **Effort:** Medium
- **Status:** ✅ **FIXED** (2026-10-27)

## Why this issue should be addressed / ranked
It is the single most-visible polish gap: every error and every destructive action pops a browser dialog that looks nothing like the Primer UI. It is ranked high among UX items because it touches the most-used paths (all actions) and directly undermines trust (a native confirm for "delete deployment" looks amateurish and is easily mis-clicked).

## Justification for the severity level
High (for UX). Not a correctness bug, but it is on every interaction and it makes the product feel unfinished. A non-blocking toast for errors and a proper in-app confirm modal (with typed or explicit confirm for destructive ops) is the expected standard.

## Dependencies
- **Depends on:** none.
- **Enables:** a reusable toast component (used by many other UX items) and a confirm-modal component (used by UX-08 loading states, which disable during the action).

## Justification for the effort level
Medium. Build a small toast (queue, auto-dismiss, success/error variants) and a confirm modal (title, body, cancel/confirm, danger styling), then replace the 9 call sites. The component work is the bulk; the call-site swaps are trivial.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT at time of audit** (re-verified 2026-10-27) — *subsequently FIXED the same day, see Resolution below*.

Evidence in `web/index.html`:

9 `alert()`/`confirm()` call sites remain (scale/terminate/delete errors + fetch-pods + save-settings). No toast or in-app confirm component exists.

## Resolution (FIXED — 2026-10-27)

Implemented two reusable, Primer-styled components and replaced all 9
`alert()`/`confirm()` call sites. No native browser dialogs remain.

### 1. Non-blocking toast (`showToast(message, {variant, title, duration})`)
- `#toastContainer` (top-right, `aria-live="polite"`) stacks toast cards.
- Variants: `success` (`check-circle`), `error` (`alert`, `role=alert`), `info`
  (`info`). Error toasts linger longer (8s) than success/info (4s).
- Pure DOM append + `setTimeout` + auto-dismiss (slide-out keyframe). Non-blocking:
  never freezes the main thread or the WS log stream (unlike `alert()`).
- Message is set via `textContent` (no HTML injection); only the (developer-
  controlled) title uses innerHTML.

### 2. In-app confirm modal (`confirmAction({title, message, detail, confirmLabel, danger})`)
- Promise-based drop-in for `confirm()`: `if (!await confirmAction({...})) return;`.
- Reuses the existing `.modal-overlay`/`.modal`/`.modal-header`/`.modal-footer`
  conventions. `#confirmModal` with danger styling (red header, red confirm button).
- Focus management: opens focused on **Cancel** (the safe button), `Escape` +
  overlay-click cancel, first resolution wins (double-click safe).
- **Delete** shows a **consequence preview** in the `detail` field
  ("This will remove `X` and its N running pod(s)") — the UX-16 hook.

### Call-site swaps (9)
- 5 errors → `showToast(..., {variant:'error'})`: scale, terminate, delete,
  fetch-pods, save (SSH key + tokens).
- 2 info → `showToast(..., {variant:'info'})`: "no pods found", "no pod for job".
- 2 destructive confirms → `await confirmAction({danger:true, ...})`: terminate,
  delete.

### Verification
- `node web/test.js` — **85 passed, 0 failed** (18 new tests for the toast
  icon/role/duration/title mapping and the confirm promise resolution semantics).
- `go build`/`go vet`/`go test ./cmd/` — all green (backend unaffected).
- **Re-deployed the whole app + all demo manifests** (`01-namespace`, `02-rbac`,
  `03-deployment`, `04-demo-workloads`, `05-security-policy`, `06-status-page`,
  `07-demo-job`); all workloads Available (deploy-dashboard 1/1, echo-server 2/2,
  ttyd-terminal 1/1, vscode-web 1/1).
- Served HTML (via port-forward to the pod) confirmed to contain the new UI
  (`toastContainer`, `confirmAction`, `function showToast`, `#confirmModal`) and
  **zero** `alert()`/`confirm()`/`prompt()` invocations.

### ⚠️ Deployment gotchas fixed along the way (see `CLUSTER-01-envoy-503.md`)
1. **Stale `//go:embed`** — the `golang:alpine` base image's warm `GOCACHE` + a
   stale buildkit `COPY --from=builder` layer made the runtime binary embed an
   OLD `web/index.html`. Fixed by `go clean -cache` + a build-time marker guard in
   the `Dockerfile`, and `--no-cache` + a post-build smoke check in
   `build-and-deploy.sh`.
2. **Mutable `:latest` tag** — `kubectl apply` alone does not roll a Deployment
   whose image tag string is unchanged. `build-and-deploy.sh` now runs
   `rollout restart` to force a re-pull.
3. **Pre-existing cluster networking flake** — after rollout, Envoy returned 503
   for `/app/` while `/apps/echo` worked (200). Root cause: the Envoy data-plane
   pod's **envoy container fails its startup probe** (`connection refused` on
   `:19003`), a node-level (brick3/brick6) pod-to-pod networking issue that
   predates this change (the original data-plane pod already showed 2 restarts).
   The dashboard pod itself is healthy and serves the new UI (verified via
   port-forward). Tracked as **CLUSTER-01**.
