# Executive Summary — Devpod / Kubernetes Deployment Dashboard

**Date:** 2026-11-11  ·  **Author:** agent session (pi)  ·  **Audience:** technical management / operators

---

## 1. One-paragraph status

The Kubernetes deployment dashboard ("Devpod") is **live, healthy, and now properly
authenticated** on the `apps01.co.slakin.net` gateway. The headline work this cycle was
a **risk-managed upgrade of the API gateway (Envoy Gateway) from the end-of-life v1.6.3
to v1.10.0** to match the cluster's Kubernetes v1.36 — a change that was overdue and
necessary, not cosmetic. During verification we **found and fixed a real authentication
gap on HTTPS**, and hit and resolved one gateway regression. The issue backlog is
essentially cleared: **33 issues closed, 5 documented as won't-fix, 11 minor
hardening items remain open** (all Low/Medium). One cluster node (`brick3`) has been
offline since 2026-11-08 with **no application impact**.

---

## 2. Current state (as of this report)

### Infrastructure
| Component | State | Notes |
|---|---|---|
| Kubernetes (k3s) | **v1.36.2** | 3 nodes |
| Nodes | brick2 ✅, brick6 ✅, **brick3 ⚠️ NotReady** | brick3 down since **2026-11-08 (~3 days)**; hosts only a MetalLB speaker — **no app impact** |
| Envoy Gateway (control plane) | **v1.10.0** ✅ | was v1.6.3 (EOL); helm release `eg` rev 2 |
| Envoy proxy (data plane) | **v1.40.2** ✅ | rolled automatically with the control plane |
| Gateway `eg` | **Programmed**, address **172.16.1.90** ✅ | MetalLB still announces despite brick3 down |
| Private registry | `registry.co.slakin.net` ✅ | reachable; **not** a Docker Hub mirror |

### Application (`apps` namespace)
| Workload | Ready | Access | Image |
|---|---|---|---|
| **deploy-dashboard** | 1/1 ✅ | `/app/` | `registry.co.slakin.net/local/deploy-dashboard:latest` |
| echo-server (demo) | 2/2 ✅ | `/apps/echo/` | `jmalloc/echo-server` |
| ttyd-terminal (demo) | 1/1 ✅ | `/apps/terminal/` | `registry.co.slakin.net/local/ttyd-web` |
| vscode-web (demo) | 1/1 ✅ | `/apps/vscode/` | `codercom/code-server` |

**Quality gates:** 66 Go unit tests + ~308 JS assertions; CI workflow + Makefile present.
Dashboard pod: 1/1, 0 restarts, **readiness + liveness probes** on `/app/healthz`.

### Security posture
- **Authentication enforced at the gateway via OIDC (dex)** on **both** HTTP and HTTPS listeners.
- **RBAC is least-privilege:** read-only cluster-scoped ClusterRole (nodes/metrics) + namespace-scoped Role; the previous cluster-wide leak is closed (SEC-10).
- Secrets moved from plaintext ConfigMap to a Secret (SEC-02); name-scoped access (SEC-03).
- Open-redirect and CORS locked down (SEC-06, SEC-07).

---

## 3. What changed this cycle (the gateway upgrade)

**Why:** Kubernetes is v1.36. Envoy Gateway v1.6.3 is **end-of-life (2026-05-13)** and only
tested to k8s v1.33 — the cluster was running it **unsupported**. v1.9/v1.10 are the first
lines that support v1.36.

**What we did (safe ordering):** snapshot baseline → upgrade CRDs first → verify gate →
upgrade controller (`--atomic`, auto-rollback on failure) → data plane rolled to v1.40.2.
Zero control-plane crashes during the upgrade itself.

**Regression found & fixed:** after the proxy moved to v1.40.2, **all traffic 404'd**. Root
cause: our custom "service starting" status page embedded CSS `100%` / `50%`; Envoy ≥1.40
parses `%` in direct-response bodies as control tokens and **rejected the entire routing
table**. Fix: removed `%` from the page. Documented as a permanent rule.

---

## 4. Notable finding — HTTPS was unauthenticated (now fixed) 🔴→✅

**Discovery:** while upgrading, we found the OIDC policy was bound only to the **HTTP (port
80)** listener. Consequence: **`https://apps01.co.slakin.net/app/` served the dashboard to
anyone with no login.** This was pre-existing (not caused by the upgrade) and defeats the
purpose of having put auth at the gateway.

**Fix:** attached the policy to the **whole gateway** (both listeners). Verified: anonymous
HTTPS now redirects to dex; the OIDC filter is on all routes for both listeners.

**Behaviour change to be aware of:** the demo workloads (`echo`, `terminal`, `vscode`) are
now **also behind login on HTTPS** (they already required it on HTTP). This closes an
anonymous-access bypass. **Decision needed:** if any of these are meant to stay public, we
should split them onto a separate unauthenticated route.

---

## 5. Outstanding items

### Infra / cluster
| Item | Severity | Impact | Recommendation |
|---|---|---|---|
| **brick3 node NotReady** (~3 days) | Med | Low today (apps healthy on 2 nodes; only a spare MetalLB speaker lost) | Investigate/recover the node or formally cordon & remove it; confirm it isn't hiding a hardware issue |
| Orphaned **`hello-world` Service** | Low | None (no pods, unreferenced) | Housekeeping — delete |
| **Envoy Gateway v1.10 → v1.11** deprecation | Low | `SecurityPolicy.targetRef` already migrated; other policies should be checked before the *next* major | Re-check release notes before next upgrade |

### Application hardening (11 open issues — all Low/Medium, none blocking)
| ID | Title | Sev | Why it matters |
|---|---|---|---|
| APP-12 | **No resource limits** on the dashboard pod | Med | A runaway container could starve the node; set CPU/mem requests+limits |
| APP-05 | No rate limiting on endpoints | Med | DoS/abuse surface (partly mitigated by gateway auth) |
| APP-07/08 | WebSocket: no keepalive/ping, no read deadline | Low | Idle/half-open WS connections may linger |
| APP-06 | Only `ReadHeaderTimeout` set | Low | Add read/write/idle timeouts (server hardening) |
| APP-09 | No structured request logging | Low | Observability gap |
| APP-14 | N+1 K8s API calls per list | Low | Efficiency; fine at current scale |
| APP-13 | No `.gitignore` | Low | Repo hygiene |
| SEC-08 | WS 1 MiB frame cap / no WS rate limit | Low | Bounded; auth-gated |

> These are **deliberate deferrals**, not unknowns — each has an issue file with severity,
> effort, and rationale. Recommend a follow-up pass to close APP-12 (resource limits) and
> APP-06 (timeouts) as the two with the best risk-reduction-per-effort ratio.

---

## 6. Known limitation (upstream bug)
Envoy Gateway still **panics** on the OIDC `passThroughAuthHeader` field even at v1.10.0
(tracked in CLUSTER-01). We keep the field **disabled**; consequence: non-browser API
clients can't present a bearer token to bypass the browser login redirect. Affects
convenience of scripted access only, not security. Revisit only when upstream confirms the
specific function is nil-safe.

---

## 7. Rollback readiness
- Gateway upgrade baselines captured (`/tmp/eg-rollback-*`: helm values/manifest/history + all gateway CRs).
- Control plane is a single `helm rollback eg 1` away from v1.6.3.
- All source-of-truth manifests + docs are **committed** (working tree clean).

---

## 8. Bottom line
**Healthy and production-ready for its current (internal, authenticated) use.** The critical
gateway upgrade is complete and stable, and we closed a genuine HTTPS auth hole in the
process. Remaining work is **operational hygiene + minor hardening**, none of it blocking:
recover or retire **brick3**, add **resource limits/timeouts** to the dashboard, and decide
whether the **demo workloads should remain public**.
