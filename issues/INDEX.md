# Issues — index & priority matrix

Each issue is a single markdown file in this directory, capturing **type**, **summary**, **severity**, **effort**, and the reasoning (why ranked, severity justification, dependencies, effort justification).

## Suggested execution order

1. **Security first (unblocks everything):** SEC-01 → SEC-02 (+ SEC-03) → SEC-04/05.
2. **Reliability backbone:** APP-02 + APP-03 + APP-12 (Deployment + probes + resources together), then APP-01/11/13 (build/embed hygiene).
3. **UX quick wins:** UX-01 (toasts + confirm modal) → UX-02 (button loading) → UX-03 (log pod/container selector) → the Low UX items.
4. **Polish & hardening:** remaining APP (rate limiting, timeouts, WS keepalive) and UX (a11y, status detail).

## Priority matrix (Severity × Effort)

| Severity | Low effort (do soon) | Medium effort (plan) | High effort (design) |
|---|---|---|---|
| **Critical** | — | — | SEC-01 |
| **High** | — | SEC-02, SEC-04, UX-01, UX-03 | — |
| **Medium** | SEC-03, APP-02, APP-03, APP-04, APP-06, APP-11, APP-13, UX-02, UX-04, UX-05, UX-07, UX-11, UX-12, UX-13, UX-15, UX-16, UX-17 | SEC-05*, APP-05, APP-07, APP-08, APP-09, APP-10, APP-14, UX-10, UX-14, UX-18 | — |
| **Low** | (see Medium column items that are Low) | | |

\* SEC-05 is Low severity / Low effort (listed here for placement; it sits in the Low band).

## By type

### Security (SEC)
| ID | Title | Severity | Effort |
|---|---|---|---|
| SEC-01 | No authentication on any endpoint | Critical | High |
| SEC-02 | Secrets stored in a plaintext ConfigMap | High | Medium |
| SEC-03 | Dashboard RBAC grants full ConfigMap CRUD (create) | Medium | Low |
| SEC-04 | Log endpoint exposes any pod, no filtering | High | Medium |
| SEC-05 | Raw K8s API error text returned to the client | Low | Low |
| SEC-06 | workload-url allows open-redirect / javascript: | Low | Low |
| SEC-07 | No CORS restriction (latent) | Low | Low |
| SEC-08 | WebSocket 1 MiB frame cap / no rate limit | Low | Medium |

### Anti-patterns (APP)
| ID | Title | Severity | Effort |
|---|---|---|---|
| APP-01 | Stale embed_gen.go — two sources of truth for the UI | Medium | Low |
| APP-02 | Dashboard runs as a raw Pod, not a Deployment | Medium | Low |
| APP-03 | No readiness or liveness probes | Medium | Low |
| APP-04 | NodePort / port 31143 hard-coded | Low | Low |
| APP-05 | No rate limiting on any endpoint | Medium | Medium |
| APP-06 | No server-level request timeouts | Medium | Low |
| APP-07 | No server-sent keepalive/ping on the WebSocket | Low | Medium |
| APP-08 | No read deadline on the WebSocket | Low | Medium |
| APP-09 | No structured/request logging | Low | Medium |
| APP-10 | Duplicated WebSocket frame codec | Low | Medium |
| APP-11 | Dockerfile copies go.sum that does not exist | Low | Low |
| APP-12 | Manifests lack resource limits, probes, controller | Medium | Low |
| APP-13 | Missing .gitignore / module hygiene | Low | Low |
| APP-14 | N+1 K8s API calls per list (no caching) | Low | Medium |

### UX
| ID | Title | Severity | Effort |
|---|---|---|---|
| UX-01 | alert()/confirm() for all feedback/confirmations | High | Medium |
| UX-02 | No loading/disabled state on action buttons | Medium | Low |
| UX-03 | Log modal silently picks first pod/container | High | Medium |
| UX-04 | Log view keeps every line in the DOM | Medium | Low |
| UX-05 | No copy or download of the log view | Medium | Low |
| UX-06 | 10 s poll continues when tab is hidden | Low | Low |
| UX-07 | Failed load renders error as a table row | Low | Low |
| UX-08 | Optimistic UI update can diverge from state | Low | Low |
| UX-09 | Theme flash on load | Low | Low |
| UX-10 | Weak accessibility (ARIA/labels) | Medium | Medium |
| UX-11 | Access-token editor fixed to 3 rows | Low | Low |
| UX-12 | SSH key shown in plaintext, no copy | Low | Low |
| UX-13 | No busy cursor during long operations | Low | Low |
| UX-14 | Merged Status column hides per-container detail | Medium | Medium |
| UX-15 | Help/Docs link points to a placeholder | Low | Low |
| UX-16 | Delete confirmation lacks detail / no preview | Medium | Low |
| UX-17 | Single theme-toggle semantics unclear | Low | Low |
| UX-18 | Table rows hide detail (no tooltip/expand) | Low | Medium |

## Key dependency chains
- **SEC-01** is the root: it mitigates SEC-04, bounds APP-05/08/14, and is the precondition for any per-user scoping.
- **APP-02 → APP-03 → APP-12**: convert to a Deployment, then add probes, then set resources (all in the same container spec).
- **SEC-02 ↔ SEC-03**: moving tokens to a Secret also changes the RBAC grant — do together.
- **APP-01 → APP-13**: the embed approach determines what `.gitignore` should cover.
- **UX-01 → UX-16**: the confirm-modal component built for toasts is reused for the enhanced delete confirm.
- **UX-03 / UX-14 / UX-18** all live in the "detail on demand" theme (logs selector, per-container status, row tooltips).
