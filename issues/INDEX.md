# Issues — index & priority matrix

Each issue is a single markdown file in this directory, capturing **type**, **summary**, **severity**, **effort**, and the reasoning (why ranked, severity justification, dependencies, effort justification).

## Suggested execution order

1. **Security first (unblocks everything):** SEC-02 (+ SEC-03). _(SEC-01, SEC-04, and SEC-05 are WONT FIX — auth is enforced by the Envoy Gateway `SecurityPolicy` OIDC+JWT, not the app; their residual scoping/sanitization is optional hardening tracked separately.)_
   - ~~SEC-02 (+ SEC-03)~~ ✅ done. ~~SEC-04~~ ⛔ WONT FIX (auth at gateway). ~~SEC-05~~ ⛔ WONT FIX (auth at gateway).
2. **Reliability backbone:** APP-02 + APP-03 + APP-12 (Deployment + probes + resources together), then APP-01/11/13 (build/embed hygiene).
3. **UX quick wins:** UX-01 (toasts + confirm modal) → UX-02 (button loading) → UX-03 (log pod/container selector) → the Low UX items.
4. **Polish & hardening:** remaining APP (rate limiting, timeouts, WS keepalive) and UX (a11y, status detail).

## Priority matrix (Severity × Effort)

| Severity | Low effort (do soon) | Medium effort (plan) | High effort (design) |
|---|---|---|---|
| **Critical** | — | — | — _(SEC-01 → WONT FIX, see Security table)_ |
| **High** | — | UX-03 | — _(SEC-04 → WONT FIX, see Security table)_ |
| **Medium** | APP-02, APP-03, APP-06, APP-13, UX-04, UX-05, UX-07, UX-11, UX-12, UX-13, UX-15, UX-16, UX-17 | APP-05, APP-07, APP-08, APP-09, APP-14, UX-10, UX-14, UX-18 | — |
| **Cluster** | — | — | — _(CLUSTER-01 resolved 2026-10-28, see below)_ |
| **Low** | (see Medium column items that are Low) | | |

\* SEC-05 is Low severity / Low effort (listed here for placement; it sits in the Low band).

## By type

### Security (SEC)
| ID | Title | Severity | Effort |
|---|---|---|---|
| SEC-01 | No authentication on any endpoint | Critical | High | ⛔ WONT FIX (gateway `SecurityPolicy` OIDC+JWT) |
| SEC-02 | Secrets stored in a plaintext ConfigMap | High | Medium | ✅ FIXED (moved to `user-settings` Secret) |
| SEC-03 | Dashboard RBAC grants full ConfigMap CRUD (create) | Medium | Low | ✅ FIXED (name-scoped Secret grant) |
| SEC-04 | Log endpoint exposes any pod, no filtering | High | Medium | ⛔ WONT FIX (auth at gateway; residual is scoping) |
| SEC-05 | Raw K8s API error text returned to the client | Low | Low | ⛔ WONT FIX (auth at gateway; bounded to vetted callers) |
| SEC-06 | workload-url allows open-redirect / javascript: | Low | Low | ✅ FIXED (same-origin-only validation + render sanitizer) |
| SEC-07 | No CORS restriction (latent) | Low | Low | ✅ FIXED (`sameOriginOnly` middleware + test) |
| SEC-08 | WebSocket 1 MiB frame cap / no rate limit | Low | Medium |
| SEC-09 | Enforce OIDC auth via an Envoy Gateway SecurityPolicy | Critical | Low | 🔧 IMPLEMENTED (dry-run; not applied live) — the fix for SEC-01 |
| SEC-10 | ClusterRole leaked namespaced grants to every namespace | High | Low | ✅ FIXED (split into read-only ClusterRole + namespaced Role; test in `tests/k8s-role-sec/`) |

### Anti-patterns (APP)
| ID | Title | Severity | Effort |
|---|---|---|---|
| APP-01 | Stale embed_gen.go — two sources of truth for the UI | Medium | Low | ✅ FIXED |
| APP-02 | Dashboard runs as a raw Pod, not a Deployment | Medium | Low | ✅ FIXED |
| APP-03 | No readiness or liveness probes | Medium | Low | ✅ FIXED |
| APP-04 | NodePort / port 31143 hard-coded | Low | Low | ✅ FIXED (script derives real URL from LB svc; stale doc refs removed) |
| APP-05 | No rate limiting on any endpoint | Medium | Medium |
| APP-06 | No server-level request timeouts | Medium | Low |
| APP-07 | No server-sent keepalive/ping on the WebSocket | Low | Medium |
| APP-08 | No read deadline on the WebSocket | Low | Medium |
| APP-09 | No structured/request logging | Low | Medium |
| APP-10 | Duplicated WebSocket frame codec | Low | Medium | ✅ FIXED (single conn/reader codec; bufio variants removed) |
| APP-11 | Dockerfile copies go.sum that does not exist | Low | Low | ✅ RESOLVED (uses `go.sum*` glob; verified via real docker build) |
| APP-12 | Manifests lack resource limits, probes, controller | Medium | Low |
| APP-13 | Missing .gitignore / module hygiene | Low | Low |
| APP-14 | N+1 K8s API calls per list (no caching) | Low | Medium |

### UX
> **Re-checked 2026-10-27:** all 18 UX issues were re-verified against the current `web/index.html`. **None have been fixed since the review — every one is still present.** Per-issue evidence is appended to each `issues/UX-*.md` under "Review — 2026-10-27".

| ID | Title | Severity | Effort | Status |
|---|---|---|---|---|
| UX-01 | alert()/confirm() for all feedback/confirmations | High | Medium | ✅ FIXED (toast + in-app confirm modal; 9 call sites swapped; 18 new tests) |
| UX-02 | No loading/disabled state on action buttons | Medium | Low | ✅ FIXED (busy-state + spinner; double-click guard; 11 unit + 12 functional tests) |
| UX-03 | Log modal silently picks first pod/container | High | Medium | ✅ FIXED + deployed + browser-verified on echo-server (2026-10-28) |
| UX-04 | Log view keeps every line in the DOM | Medium | Low | 🔴 still present (appendChild per line, no cap) |
| UX-05 | No copy or download of the log view | Medium | Low | 🔴 still present (log modal footer has no Copy/Download) |
| UX-06 | 10 s poll continues when tab is hidden | Low | Low | 🔴 still present (setInterval, no visibilitychange gate) |
| UX-07 | Failed load renders error as a table row | Low | Low | 🔴 still present (catch injects a pseudo-row) |
| UX-08 | Optimistic UI update can diverge from state | Low | Low | 🔴 still present (optimistic mutation + delayed refresh) |
| UX-09 | Theme flash on load | Low | Low | 🔴 still present (no inline head script; setTheme after paint) |
| UX-10 | Weak accessibility (ARIA/labels) | Medium | Medium | 🔴 still present (1 aria-label total; icon buttons unlabeled) |
| UX-11 | Access-token editor fixed to 3 rows | Low | Low | 🔴 still present (3 hardcoded rows, no add/remove) |
| UX-12 | SSH key shown in plaintext, no copy | Low | Low | 🔴 still present (plaintext, no copy button; title already says 'Public Key') |
| UX-13 | No busy cursor during long operations | Low | Low | 🔴 still present (no cursor:wait/progress bar) |
| UX-14 | Merged Status column hides per-container detail | Medium | Medium | 🔴 still present (single badge, no per-container detail) |
| UX-15 | Help/Docs link points to a placeholder | Low | Low | 🔴 still present (href → example.co.slakin.net) |
| UX-16 | Delete confirmation lacks detail / no preview | Medium | Low | 🔴 still present (generic native confirm) |
| UX-17 | Single theme-toggle semantics unclear | Low | Low | 🔴 still present (icon reflects action, not current theme; no title) — *borderline* |
| UX-18 | Table rows hide detail (no tooltip/expand) | Low | Medium | 🔴 still present (3 columns, no tooltip/expand) |

## Cluster / infrastructure
| ID | Title | Severity | Effort | Status |
|---|---|---|---|---|
| CLUSTER-01 | Envoy 503 — OIDC `passThroughAuthHeader` crashes v1.6.3 xDS translator | High | Low | ✅ RESOLVED (2026-10-28: removed the crashing field + obsolete `ws-route`; proper fix = gateway upgrade) |

## Key dependency chains
- **SEC-01 (WONT FIX) → SEC-09**: the in-app gap is closed by the Envoy Gateway `SecurityPolicy` OIDC provider (`manifests/05-security-policy.yaml`); SEC-09 is that implementation.
- **SEC-01** (WONT FIX) was the root: in-app auth is out of scope because the Envoy Gateway `SecurityPolicy` enforces OIDC+JWT in front of the app. Its mitigation effect on SEC-04 and APP-05/08/14 now comes from the gateway, not the app.
- **APP-02 → APP-03 → APP-12**: convert to a Deployment, then add probes, then set resources (all in the same container spec).
- **SEC-02 ↔ SEC-03**: moving tokens to a Secret also changes the RBAC grant — do together.
- **APP-01 → APP-13**: the embed approach determines what `.gitignore` should cover.
- **UX-01 → UX-16**: the confirm-modal component built for toasts is reused for the enhanced delete confirm.
- **UX-03 / UX-14 / UX-18** all live in the "detail on demand" theme (logs selector, per-container status, row tooltips).

## UX re-check (2026-10-27)

All 18 UX issues were re-verified against the current `web/index.html`. **UX-01
was fixed on 2026-10-27** (toast + in-app confirm modal); the other 17 remain
open. Highlights:

- **High:** ~~UX-01~~ (fixed 2026-10-27), UX-03 (log modal silently picks the first ready pod + `containers[0]`, no selector).
- **Medium:** ~~UX-02~~ (fixed 2026-10-27), UX-04/05 (unbounded log DOM; no
  copy/download), UX-10 (a single `aria-label` in the whole file), UX-14 (merged
  status hides per-container detail), UX-16 (delete has no consequence preview).
- **Low:** UX-06/07/08/09/11/12/13/15/17/18 all confirmed present.

No code changes were made in this pass — it is a status audit only. The per-issue
evidence lines are appended to each `issues/UX-*.md` file.
