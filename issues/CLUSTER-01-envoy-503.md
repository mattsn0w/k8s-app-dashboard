# CLUSTER-01: Envoy Gateway 503 — OIDC SecurityPolicy crashes the v1.6.3 xDS translator

- **Type:** Infrastructure / Cluster (Envoy Gateway control plane)
- **Summary:** `GET https://apps01.co.slakin.net/app/` returned `HTTP 503`
  ("upstream connect error ... No route to host") while other routes (e.g.
  `/apps/echo`) returned 200. Root cause: the **OIDC `SecurityPolicy`
  `apps01-oidc` crashed the Envoy Gateway **v1.6.3** control plane's xDS
  translator** with a nil-pointer dereference, which crash-looped the xDS
  runner so the data plane never received its config and failed its startup
  probe (`connection refused` on `:19003`).
- **Severity:** High (broke the public dashboard route)
- **Effort:** Low (config change) / Medium (proper fix = upgrade the gateway)
- **Status:** ✅ **RESOLVED** (2026-10-28) — gateway recovered, OIDC auth
  restored (minus `passThroughAuthHeader`), obsolete `ws-route` removed.

## Root cause (confirmed by elimination)

**Trigger:** the OIDC field **`passThroughAuthHeader: true`**.

The control-plane (`envoy-gateway`) log showed a recurring panic:

```
ERROR watchable ... observed a panic  {"runner": "xds", ...}
panic: runtime error: invalid memory address or nil pointer dereference
github.com/envoyproxy/gateway/internal/xds/translator.buildHeaderMatchers(0x0)
    .../internal/xds/translator/oidc.go:323
... buildHCMOAuth2Filter → patchHCMWithFilters
```

`buildHeaderMatchers` was called with a nil argument (`0x0`). `passThroughAuthHeader`
is the OIDC field that wires the JWT pass-through filter into the OAuth2 HCM
translation, and in **v1.6.3** that path dereferences a nil. Each panic killed the
xDS translation for the gateway, so the data plane's gRPC config fetch timed out:

```
[envoy data plane] gRPC config: initial fetch timed out for
  type.googleapis.com/envoy.config.listener.v3.Listener (and Cluster, ClusterLoadAssignment)
```

With no listener config, the data plane never bound `:19003` (the admin/ready
port the startup probe hits), so the probe got `connection refused` and the pod
was stuck at 1/2 → Envoy returned 503 for the routes it couldn't program.

### Elimination (empirical, on the live cluster)
| OIDC policy variant | panics | `/app/` |
|---|---|---|
| policy deleted | 0 | 200 |
| OIDC **without** `passThroughAuthHeader`/`refreshToken` | 0 | 200 |
| OIDC **with only** `passThroughAuthHeader: true` | **1** | 200 (transient) |
| OIDC with the full original spec (incl. `passThroughAuthHeader`) | 8 (looping) | 503 |

Conclusion: **`passThroughAuthHeader: true` is the exact trigger.** This was an
Envoy Gateway v1.6.3 bug. It was originally believed fixed in later versions,
but that was **DISPROVEN on 2026-11-11**: after upgrading the cluster to
**v1.10.0**, setting `passThroughAuthHeader: true` reproduces the *identical*
panic at `xds/translator/oidc.go:395 buildHeaderMatchers(0x0)` → nil pointer.
So the version bump is **NOT** a fix for this specific field. See §"Update
2026-11-11" at the bottom for the corrected status.

## Secondary (pre-existing, unrelated) issue found during investigation

The live-only **`ws-route`** HTTPRoute (not in any manifest) was broken since
2026-10-07:
- `parentRefs[].sectionName: ws` — the Gateway `eg` has **no `ws` listener**
  (only `http:80`, `https:443`) → `NoMatchingParent`.
- `backendRefs[].port: 8081` — the `deploy-dashboard` Service only has
  `http:8080` → `PortNotFound: "TCP Port 8081 not found"`.
- It emitted `setting 500 direct response in routes due to errors in processing
  destinations` in the control plane.

It was a leftover from an earlier design where the WS endpoint lived at a
different path/port. The current SPA uses `wss://<host>/app/api/logs-ws`, which is
served by the main `/app` PathPrefix route (verified: a real WebSocket handshake
through the gateway returned `101 Switching Protocols`). **Deleted** the obsolete
`ws-route`; the control-plane errors stopped.

## Resolution applied (2026-10-28)

1. **Removed the crashing OIDC field** — `manifests/05-security-policy.yaml`
   no longer sets `passThroughAuthHeader: true` (with a comment documenting the
   bug and the proper fix). Re-applied; the control plane stopped panicking and
   the data plane recovered (2/2). `/app/` → 200, all routes healthy.
   - **Trade-off:** non-browser clients (curl, `kubectl port-forward`) can no
     longer present `Authorization: Bearer <token>` to bypass the OIDC redirect.
     Browser login still works.
2. **Deleted the obsolete `ws-route`** (live-only leftover, not in a manifest).

## Verified healthy after the fix
- `observed a panic` count in the control plane: **0**.
- `/app/` → 200 (serving the current UI; `toastContainer` + `busyActions` present).
- `/apps/echo` → 200; `/apps/terminal`, `/apps/vscode/` → 302 (expected redirects).
- WebSocket `/app/api/logs-ws` upgrade → **101 Switching Protocols**.
- No more `Port 8081` / `NoMatchingParent` / `500 direct response` errors.

## Proper / permanent fix (recommended, not yet applied)

**Upgrade the Envoy Gateway** from `v1.6.3` to a version that has the OIDC
`buildHeaderMatchers` nil guard (v1.8.x+; v1.9.x confirmed present in the local
source checkout). After the upgrade, re-add `passThroughAuthHeader: true` to
restore Bearer-token pass-through for non-browser clients. Steps:
1. Bump the Envoy Gateway control-plane deployment image
   (`envoy-gateway-system/envoy-gateway`) to the target version.
2. Roll out the matching data-plane (the `SecurityPolicy`/gateway CRDs are
   versioned; follow the Envoy Gateway upgrade notes for the data plane).
3. Re-apply `manifests/05-security-policy.yaml` **with** `passThroughAuthHeader: true`.

> ⚠️ **This recommendation was WRONG and is superseded.** The gateway was
> upgraded to **v1.10.0** on 2026-11-11 and `passThroughAuthHeader` STILL
> panics there. See the update below — do NOT re-add the field without first
> verifying `buildHeaderMatchers` is nil-safe in the *running* image.

---

## Update 2026-11-11 (after upgrading to Envoy Gateway v1.10.0)

**The upgrade did NOT fix `passThroughAuthHeader`.** On the now-running
**v1.10.0** control plane, setting `passThroughAuthHeader: true` reproduces the
exact same panic:

```
panic: runtime error: invalid memory address or nil pointer dereference
  internal/xds/translator.buildHeaderMatchers(0x0)
    internal/xds/translator/oidc.go:395
  internal/xds/translator.oauth2Config(...)
    internal/xds/translator/oidc.go:217
```

Difference from v1.6.3: in v1.10.0 `message.handleWithCrashRecovery` **catches**
the panic, so the process does **not** crash-loop and restartCount stays 0 —
but the OIDC xDS translation is dropped (the setting never takes effect) and
the xDS runner logs `observed a panic` on every reconcile. Net: the field is
still unusable; the workaround (omit it) remains in effect.

**Status update:** ✅ still RESOLVED via workaround (field omitted). The
"permanent fix = upgrade" note above was incorrect and is struck through.
**Re-open criteria:** only revisit `passThroughAuthHeader` when
`buildHeaderMatchers` is confirmed nil-safe in the running EG image (read the
release notes / source for that exact function), not merely on a version bump.
Tracked in docs/envoy-gateway-upgrade-plan.md §10b.
4. Verify: 0 panics, `/app/` → 200, and a Bearer-token request bypasses the
   login redirect.
