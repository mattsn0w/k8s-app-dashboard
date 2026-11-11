#!/usr/bin/env bash
# Envoy Gateway v1.6.3 -> v1.10.x upgrade — RUNBOOK (execute on a host WITH Docker Hub egress)
#
# WHY here: this sandbox cannot download the chart/images. Confirmed blocked (403/000):
#   - helm OCI pull -> "Forbidden"
#   - registry-1.docker.io blob -> 307 -> production.cloudfront.docker.com -> 403
#   - k8s nodes: registry-1.docker.io / cloudfront / auth.docker.io all 403/FAIL
#   - private mirror registry.co.slakin.net does NOT have envoyproxy/* (NAME_UNKNOWN)
#   - github release assets (objects.githubusercontent.com) -> blocked (000)
# So run these steps from brick2 (the node) or any host that can reach Docker Hub.
#
# Pinned target: v1.10.0 (verified present via Docker Hub manifest API: HTTP 200).
# Cluster: k3s v1.36.2. Current: Helm release "eg", chart gateway-helm-v1.6.3, ns envoy-gateway-system.
# Baseline already captured at /tmp/eg-rollback-20261111-0819 (values=null, revision 1 only).
#
# The hard rule: CRDs FIRST, then the controller. Verify the gate before the controller bump.

set -euo pipefail
export KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
VER=1.10.0          # exact patch; bump if a newer 1.10.x exists (check helm show / hub tags)
CH=experimental     # current channel; keep it

echo "== [0] baseline (idempotent; already done in sandbox) =="
BK=/tmp/eg-rollback-$(date +%Y%m%d)
mkdir -p "$BK"
helm get values eg -n envoy-gateway-system > "$BK/values.yaml"
helm get manifest eg -n envoy-gateway-system > "$BK/manifest.yaml"
helm history eg -n envoy-gateway-system > "$BK/history.txt"
kubectl get gatewayclass,gateway,httproute,referencegrant,securitypolicy -A -o yaml > "$BK/gateway-crs.yaml"
echo "baseline -> $BK"

echo "== [1] upgrade CRDs (Gateway API $CH $VER + EG CRDs). CRDs BEFORE controller. =="
# Channel must stay experimental (matches today). Use gateway-crds-helm chart.
helm template eg-crds oci://docker.io/envoyproxy/gateway-crds-helm \
  --version "$VER" \
  --set crds.gatewayAPI.enabled=true \
  --set crds.gatewayAPI.channel="$CH" \
  --set crds.envoyGateway.enabled=true \
  | kubectl apply --force-conflicts --server-side -f -

echo "== [2] GATE: verify CRDs landed before touching the controller =="
kubectl get crd gateways.gateway.networking.k8s.io \
  -o go-template='bundle={{ index .metadata.annotations "gateway.networking.k8s.io/bundle-version" }} channel={{ index .metadata.annotations "gateway.networking.k8s.io/channel" }}{{"\n"}}'
echo "  ^ expect bundle=v1.6.x channel=experimental before continuing"
echo "ReferenceGrant served versions (drives step 3):"
kubectl get crd referencegrants.gateway.networking.k8s.io \
  -o jsonpath='served={.spec.versions[*].name} stored={.status.storedVersions}{"\n"}'
read -rp "CRDs look right (bundle v1.6.x)? continue? [y/N] " a; [[ "$a" == y ]] || { echo "halted"; exit 1; }

echo "== [3] (only if ReferenceGrant v1beta1 is NO LONGER served) bump our manifest to v1 =="
# Our live ReferenceGrant uses gateway.networking.k8s.io/v1beta1. If the new bundle serves only v1:
#   sed -i 's#gateway.networking.k8s.io/v1beta1#gateway.networking.k8s.io/v1#' manifests/03-deployment.yaml
#   kubectl -n apps apply -f manifests/03-deployment.yaml
echo "  (inspect the step-2 output; skip if v1beta1 still served)"

echo "== [4] upgrade the CONTROLLER to $VER (values are null, nothing to re-pass) =="
helm upgrade eg oci://docker.io/envoyproxy/gateway-helm --version "$VER" -n envoy-gateway-system

echo "== [5] wait + confirm control plane =="
kubectl -n envoy-gateway-system rollout status deploy/envoy-gateway --timeout=5m
kubectl -n envoy-gateway-system get deploy envoy-gateway -o jsonpath='image={.spec.template.spec.containers[0].image}{"\n"}'
echo "  ^ expect ...gateway:v1.10.0"
kubectl -n envoy-gateway-system logs deploy/envoy-gateway --tail=50 | grep -iE "panic|nil pointer|error" && echo "!! errors in EG logs" || echo "EG logs clean"

echo "== [6] data plane rolls to v1.40.x proxy =="
kubectl -n envoy-gateway-system get pods -o wide | grep -i envoy-apps
kubectl -n apps get gateway,eg -o wide   # expect PROGRAMMED True, ADDRESS 172.16.1.90

echo "== [7] e2e =="
curl -sk -o /dev/null -w "dashboard /app/: HTTP %{http_code}\n" https://apps01.co.slakin.net/app/  # 200 (or 302 to OIDC if logged out)
echo "Manually verify: OIDC login (expect ONE re-login), WebSocket logs (101), /apps/vscode/, ttyd terminal."

echo "== [8] OPTIONAL now that CLUSTER-01 crash is fixed upstream: restore passThroughAuthHeader =="
# Re-add passThroughAuthHeader: true to manifests/05-security-policy.yaml, apply, watch EG logs for panics,
# then mark issues/CLUSTER-01-envoy-503.md 'fixed upstream v1.8.4+; workaround removed'.

echo "== ROLLBACK (if needed): =="
echo "  helm rollback eg 1 -n envoy-gateway-system   # back to v1.6.3 controller (CRDs stay v1.6, compatible with our resources)"
