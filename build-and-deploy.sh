#!/bin/bash
# Build and deploy the deploy-dashboard to the k3s cluster.
#
# Usage:
#   ./build-and-deploy.sh
#
# The dashboard runs as a Deployment (see manifests/03-deployment.yaml). The
# binary (with the UI embedded via //go:embed) is baked into the image, so this
# script just:
#   1. Builds the image (docker build, multi-stage: static Go binary in alpine)
#   2. Pushes it to the local registry
#   3. Applies the manifests (the Deployment rolls to the new image)
#   4. Waits for the rollout to be ready
#
# No binary injection, no emptyDir, no initContainer -- a recreated pod is
# self-sufficient because its image already contains the binary.
set -euo pipefail

cd "$(dirname "$0")"

# Config: image + registry + namespace.
IMAGE="${IMAGE:-registry.co.slakin.net/local/deploy-dashboard:latest}"
NS="apps"
DEPLOY="deploy-dashboard"

echo "=== 1. Build image: $IMAGE ==="
docker build -t "$IMAGE" .

echo ""
echo "=== 2. Push image ==="
docker push "$IMAGE"

echo ""
echo "=== 3. Apply manifests (Deployment rolls to the new image) ==="
kubectl apply -f manifests/03-deployment.yaml

echo ""
echo "=== 4. Wait for rollout ==="
kubectl -n "$NS" rollout status "deployment/$DEPLOY" --timeout=180s

echo ""
echo "=== Done ==="
kubectl -n "$NS" get pods -l "app.kubernetes.io/name=$DEPLOY"

# Print the REAL access URL, derived from the gateway's LoadBalancer Service +
# the Gateway hostname (no hard-coded ports/IPs -- see APP-04). The Envoy
# Gateway creates one LoadBalancer Service per Gateway object; we find it and
# read its EXTERNAL-IP, then pair it with the Gateway listener hostname.
GATEWAY_NS="apps"
GATEWAY_NAME="eg"
echo ""
echo "=== Access URL (derived from the gateway LoadBalancer Service) ==="
# Find the LoadBalancer Service the gateway created (name prefix "envoy-").
LB_SVC=$(kubectl -n envoy-gateway-system get svc -o json 2>/dev/null \
  | python3 -c "import sys,json; svcs=json.load(sys.stdin)['items']; print(next((s['metadata']['name'] for s in svcs if s['spec'].get('type')=='LoadBalancer'),''))" 2>/dev/null || true)
HOSTNAME_=$(kubectl -n "$GATEWAY_NS" get gateway "$GATEWAY_NAME" -o jsonpath='{.spec.listeners[0].hostname}' 2>/dev/null || true)
if [ -n "$LB_SVC" ] && [ -n "$HOSTNAME_" ]; then
  LB_IP=$(kubectl -n envoy-gateway-system get svc "$LB_SVC" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
  echo "  Gateway hostname : $HOSTNAME_"
  echo "  LoadBalancer IP  : ${LB_IP:-<pending>}"
  echo "  Access via       : https://$HOSTNAME_/   (or http://$HOSTNAME_/ which redirects to HTTPS)"
  if [ -n "$LB_IP" ]; then
    echo "  (raw LB IP)      : https://$LB_IP/ -- host header must be $HOSTNAME_"
  fi
else
  echo "  (could not derive URL; check the gateway + its LoadBalancer Service)"
fi
