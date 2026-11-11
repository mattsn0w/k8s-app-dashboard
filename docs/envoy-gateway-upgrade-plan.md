# Envoy Gateway Upgrade Plan — v1.6.3 → v1.10.x

**Cluster:** k3s, Kubernetes **v1.36.2**
**Current Envoy Gateway:** **v1.6.3** (Helm release `eg`, chart `gateway-helm-v1.6.3`, ns `envoy-gateway-system`)
**Target Envoy Gateway:** **v1.10.x** (latest patch of the v1.10 line)
**Gateway API CRDs today:** bundle **v1.4.1**, **experimental** channel
**Date:** 2026-11-11

---

## 0. Why upgrade now (grounded in the compatibility matrix)

From the vendored **Version Compatibility Matrix** (`news/releases/matrix.md`):

| Envoy Gateway | Gateway API | Kubernetes supported | EOL |
|---|---|---|---|
| **v1.6** (ours) | v1.4.0 | **v1.30–v1.33** | 2026-05-13 |
| v1.7 | v1.4.1 | v1.32–v1.35 | 2026-08-05 |
| v1.8 | v1.5.1 | v1.32–v1.35 | 2026-11-08 |
| v1.9 | v1.6.1 | v1.33–v1.36 | 2027-02-14 |
| **v1.10** (target) | **v1.6.1** | **v1.34–v1.37** | 2027-05-05 |

Key conclusions:

- **Our cluster (v1.36) is outside v1.6's tested range (max v1.33)** and v1.6 is **past EOL (2026-05-13)**. The upgrade is overdue, not merely nice-to-have.
- **v1.7 and v1.8 top out at Kubernetes v1.35** — they do **not** support v1.36. So we should **not** treat them as destinations, and there is no reason to linger on them.
- **v1.9 and v1.10 both support v1.36.** v1.10 is the newest line (EOL 2027-05-05) and additionally fixes the **SecurityPolicy OIDC nil-pointer** class of crashes (see §1) — so **v1.10 is the target.**

> This upgrade also permanently resolves **CLUSTER-01** (the v1.6.3 OIDC `passThroughAuthHeader` control-plane panic we had to work around by removing the field). v1.8.4+/v1.9.1+/v1.10.0 release notes carry the corresponding "Fixed a nil pointer dereference in SecurityPolicy translation" fix.

---

## 1. Breaking-change review against **our** resources

Reviewed the v1.7.0 → v1.10.0 release notes against the live config. Items relevant to us:

| Change (version) | Affects us? | Action |
|---|---|---|
| **Gateway API v1.6 required at v1.9**; `TCPRoute`/`UDPRoute` move `v1alpha2`→`v1` | **No — we have zero TCPRoute/UDPRoute objects** (`kubectl get tcproutes,udproutes -A` → none). | Still upgrade CRDs to v1.6 first (mandatory ordering). No route rewrites needed. |
| **v1.9** safe-upgrades `ValidatingAdmissionPolicy` moved from CRD bundle → chart templates | **Yes (ordering only).** We install CRDs via the umbrella chart (not separately), so Helm manages it. | Covered by the CRDs-then-controller order in §3. |
| **v1.9** Lua disabled by default (`enableLua`) | No — no `EnvoyExtensionPolicy`/Lua in our manifests. | None. |
| **v1.10** OIDC issuer must not be `http://` | **No — our issuer is `https://dex.co.slakin.net/auth`.** | None. |
| **v1.10** SecurityPolicy `authorization` referencing an undefined JWT provider → 500 | **No — we use OIDC only; no `authorization` block.** | None. |
| **v1.10** OIDC session cookies now AES-256-GCM (old CBC sessions rejected) | **Yes — cosmetic.** | Users with an active session are redirected to re-auth **once**. Acceptable; schedule in a low-traffic window. |
| **v1.10** OIDC **token cookie chunking** (new default), proxies **< v1.40 can't read split cookies** | **Yes — during the mixed-version proxy rollout only.** | Keep the rollout clean (see §5). The control plane and proxies upgrade together via the chart, so the window is short. If any old data plane lingers, disable via `runtimeFlags.disabled: [OIDCTokenCookieChunking]`. |
| **v1.10** JWT requirement rename → transient 403 `Wrong requirement_name` on long-lived connections | **No — we don't use JWT `SecurityPolicy`.** | None. |
| **v1.10** session-persistence cookie `Path=/`; generated session names get a hash suffix | No — no session persistence configured. | None. |
| ReferenceGrant | Live CRD serves **only `v1beta1`**; our manifest uses `gateway.networking.k8s.io/v1beta1`. Gateway API promotes ReferenceGrant to GA (`v1`) in a later bundle. | **Verify the new Gateway API v1.6 bundle still serves `v1beta1` for ReferenceGrant** (§6 check). If GA-only, bump our manifest to `v1` (trivial, one-line). Do this as part of CRD apply, before controller bump. |

**Net:** our specific footprint (HTTPRoute + Gateway + SecurityPolicy-OIDC + one ReferenceGrant, no TCP/UDP/JWT/Lua/session-persistence) is **clean** against v1.10. The only user-visible effect is a one-time OIDC re-login.

---

## 2. Upgrade mechanism (Helm; single node, single EG replica)

Helm does **not** upgrade CRDs in `crds/`, so the rule from the docs is absolute: **upgrade CRDs first, then the controller**, or the controller can't reconcile the new resource versions.

- **Do not** `helm uninstall` (that would delete the Gateway data plane → downtime).
- A **direct 1.6.3 → 1.10.x** `helm upgrade` is acceptable for the control plane (the chart is designed to be upgraded to the latest line; we skip 1.7/1.8 deliberately since they don't support v1.36). The CRDs are applied via server-side `kubectl apply` (idempotent, handles the v1.4.1→v1.6.1 jump in one pass).

> WARNING **Verify chart tag existence from a network-enabled host first** (the sandbox here can't reach Docker Hub OCI). The chart is `oci://docker.io/envoyproxy/gateway-helm`; confirm the exact latest patch tag (e.g. `1.10.0`, `1.10.x`) with:
> `helm show chart oci://docker.io/envoyproxy/gateway-helm --version 1.10.0` and check Docker Hub tags for `envoyproxy/gateway-helm` and `envoyproxy/gateway-crds-helm`.
> Pin an **exact** patch version (no floating tags) in every command below as `1.10.PATCH`.

---

## 3. Procedure

Capture a rollback baseline first (§4), then:

### Step A — Upgrade CRDs (Gateway API v1.6.1 + Envoy Gateway CRDs), same experimental channel
```bash
export KUBECONFIG=~/.kube/config
# Experimental channel to match the current install (keeps v1alpha2 served).
helm template eg-crds oci://docker.io/envoyproxy/gateway-crds-helm \
  --version 1.10.PATCH \
  --set crds.gatewayAPI.enabled=true \
  --set crds.gatewayAPI.channel=experimental \
  --set crds.envoyGateway.enabled=true \
  | kubectl apply --force-conflicts --server-side -f -
```
Verify CRDs landed (see §6) **before** proceeding.

### Step B — (if needed) migrate ReferenceGrant to a served version
Only if the check in §6 shows ReferenceGrant `v1beta1` is no longer served in the v1.6 bundle:
```bash
# One-line bump in manifests/03-deployment.yaml, then:
kubectl -n apps apply -f manifests/03-deployment.yaml   # ReferenceGrant section
```

### Step C — Upgrade the controller (Helm) to v1.10.PATCH
```bash
helm upgrade eg oci://docker.io/envoyproxy/gateway-helm \
  --version 1.10.PATCH \
  -n envoy-gateway-system
# Preserve any custom values from the current release:
#   helm get values eg -n envoy-gateway-system   # run first; re-pass with -f if non-empty
```

### Step D — Wait + confirm control plane
```bash
kubectl -n envoy-gateway-system rollout status deploy/envoy-gateway --timeout=5m
kubectl -n envoy-gateway-system get pod -l app.kubernetes.io/name=gateway-helm
kubectl -n envoy-gateway-system logs deploy/envoy-gateway --tail=50 | grep -iE "error|panic|nil pointer" || echo "clean"
```

### Step E — Restart / reconcile the data plane
The Gateway deployment `envoy-apps-eg-8aba2fe6` runs the **Envoy proxy** image. EG re-generates it on control-plane version change; confirm it rolls to the v1.40.x-based proxy:
```bash
kubectl -n envoy-gateway-system get deploy -l gateway.envoyproxy.io/owning-gateway-name
kubectl -n envoy-gateway-system get pods -o wide | grep envoy-apps   # should show the new proxy, all Ready
```

---

## 4. Rollback baseline (capture **before** touching anything)

```bash
export KUBECONFIG=~/.kube/config
BK=eg-rollback-$(date +%Y%m%d-%H%M)
mkdir -p /tmp/$BK
# 1. Current Helm values + manifest (restore target for the control plane)
helm get values eg -n envoy-gateway-system > /tmp/$BK/values.yaml
helm get manifest eg -n envoy-gateway-system > /tmp/$BK/manifest.yaml
helm history eg -n envoy-gateway-system > /tmp/$BK/history.txt
# 2. All gateway/envoy CRs (data/config to restore if a CR was mutated)
kubectl get gatewayclass,gateway,httproute,referencegrant,securitypolicy,backendtrafficpolicy -A -o yaml > /tmp/$BK/gateway-crs.yaml
# 3. Current Gateway API CRD bundle version (recorded above: v1.4.1, experimental)
echo "bundle=v1.4.1 channel=experimental" > /tmp/$BK/crd-info.txt
```

**Rollback (control plane):** `helm rollback eg <prev-rev> -n envoy-gateway-system` (redeploys v1.6.3).
**Note:** Gateway API CRDs are **not** downgraded by Helm rollback. v1.6 CRDs remain compatible with the v1.6.3 controller for the resources we use (HTTPRoute/Gateway/ReferenceGrant/SecurityPolicy all existed at v1.4/v1.6 with the same/served versions), so a control-plane rollback is safe without a CRD downgrade. If you must downgrade CRDs, re-apply the v1.4.1 experimental bundle from `gateway-crds-helm --version 1.6.3`.

---

## 5. Verification & post-upgrade checklist

**Gateway/traffic (§6 for full command list):**
- [ ] `kubectl -n apps get gateway,eg` -> `PROGRAMMED=True`, address `172.16.1.90`.
- [ ] `curl` the dashboard: `https://apps01.co.slakin.net/app/` -> **200** (or **302** to OIDC when logged out).
- [ ] OIDC login round-trips once; session persists afterward (GCM cookies). **Expected: one re-login.**
- [ ] WebSocket log streaming still upgrades (`101`) — this is our primary regression surface after CLUSTER-01.
- [ ] Each workload link works: `/apps/vscode/` (URLRewrite filter intact), echo-server, ttyd terminal.
- [ ] **Optional — restore `passThroughAuthHeader`** now that the crash is fixed: re-add it to `manifests/05-security-policy.yaml`, apply, confirm 0 panics in EG logs, and update CLUSTER-01 to "fixed upstream in v1.8.4+; workaround removed."
- [ ] Gateway logs clean: `kubectl -n envoy-gateway-system logs deploy/envoy-apps-eg-8aba2fe6 --tail=50 | grep -iE "error|panic"`.

**Mixed-version window (short):** if any old (<v1.40) Envoy proxy lingers beside the new control plane while serving `apps01`, disable token-cookie-chunking to avoid the split-cookie read failure:
```bash
helm upgrade eg oci://docker.io/envoyproxy/gateway-helm --version 1.10.PATCH \
  -n envoy-gateway-system --set 'config.envoyGateway.runtimeFlags.disabled={OIDCTokenCookieChunking}'
```
Re-enable (drop the flag) once all proxies are on the new image. On a single-node cluster with one Gateway this is normally unnecessary — verify proxies are uniform first.

---

## 6. Verification commands

```bash
export KUBECONFIG=~/.kube/config NO_PROXY=127.0.0.1,::1,localhost

# Gateway API bundle now at v1.6.x?
kubectl get crd gateways.gateway.networking.k8s.io \
  -o go-template='version={{ index .metadata.annotations "gateway.networking.k8s.io/bundle-version" }} channel={{ index .metadata.annotations "gateway.networking.k8s.io/channel" }}{{ "\n" }}'

# ReferenceGrant still serves the version our manifest uses? (drives Step B)
kubectl get crd referencegrants.gateway.networking.k8s.io \
  -o jsonpath='served={.spec.versions[*].name} stored={.status.storedVersions}{"\n"}'

# Control-plane version label bumped to v1.10.x?
kubectl -n envoy-gateway-system get deploy envoy-gateway -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'

# Gateway programmed + address preserved (MetalLB 172.16.1.90)?
kubectl -n apps get gateway,eg -o wide

# EG data-plane image moved to the v1.40.x proxy line?
kubectl -n envoy-gateway-system get pods -l app.kubernetes.io/component=proxy \
  -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.containers[0].image}{"\n"}{end}'

# End-to-end (auth-gated):
curl -sk -o /dev/null -w "%{http_code}\n" https://apps01.co.slakin.net/app/
```

---

## 7. Risk register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| CRDs upgraded after controller -> controller can't reconcile | Low | High (traffic drop) | Strict ordering in §3; §6 CRD check gate before Step C |
| ReferenceGrant `v1beta1` not served post-CRD | Low | Low (cross-ns backend ref breaks) | Step B one-line bump to `v1` |
| Mixed-version proxies can't read chunked OIDC cookies | Low | Medium (login loop briefly) | Uniform-proxy verify; `runtimeFlags.disabled` fallback |
| Custom Helm values lost on upgrade | Medium | Medium | `helm get values` first, re-pass with `-f` |
| Skipping 1.7/1.8 introduces a latent incompat | Low | Medium | Our resource set is verified against every 1.7–1.10 breaking note (§1); v1.7/1.8 don't support v1.36 anyway |

---

## 8. Recommended execution order (one-liner)
1. §4 baseline snapshot -> 2. §2 tag verification -> 3. §3 Step A (CRDs) -> 4. §6 CRD/ReferenceGrant check -> 5. §3 Step B if needed -> 6. §3 Step C/D (controller) -> 7. §3 Step E (data plane) -> 8. §5/§6 verify -> 9. optional `passThroughAuthHeader` restore + CLUSTER-01 closeout.
