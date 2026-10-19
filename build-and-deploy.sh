#!/bin/bash
# Build and deploy the deploy-dashboard to the k3s cluster.
#
# Usage:
#   ./build-and-deploy.sh [kubeconfig-path]
#
# The dashboard runs as a Deployment (see manifests/03-deployment.yaml). Its
# pod blocks in an initContainer until the static binary is injected into the
# shared emptyDir, so this script:
#   1. Builds a static Go binary (the UI is embedded in the binary)
#   2. Applies the Deployment manifest (pod starts, initContainer blocks)
#   3. Injects the binary into the pod via a tar stream
#   4. Waits for the pod to pass its readiness probe
#   5. Verifies via port-forward
set -euo pipefail

KUBECONFIG="${1:-$HOME/.kube/config}"
export KUBECONFIG
NS="apps"
DEPLOY="deploy-dashboard"
# Match the Deployment-managed pod by its name prefix (not the raw-Pod name).
POD_LABEL="app.kubernetes.io/name=deploy-dashboard"

echo "=== 1. Build static binary ==="
cd "$(dirname "$0")"
CGO_ENABLED=0 go build -ldflags="-s -w -extldflags=-static" -o /tmp/deploy-dashboard-static ./cmd/
echo "Binary: $(ls -lh /tmp/deploy-dashboard-static | awk '{print $5}')"
SHA_LOCAL=$(sha256sum /tmp/deploy-dashboard-static | awk '{print $1}')

echo ""
echo "=== 2. Apply Deployment ==="
# Applying the manifest creates the Deployment if absent. The pod it spawns is
# stuck in the initContainer (waiting for the binary) until step 3.
kubectl apply -f manifests/03-deployment.yaml

echo ""
echo "=== 3. Find the Deployment pod ==="
# The Deployment pod's name is <deployment>-<hash>-<id>; the raw "dashboard-runner"
# Pod (if any lingers) does not carry that prefix.
DEPPOD=""
for _ in $(seq 1 30); do
  DEPPOD=$(kubectl -n "$NS" get pods -o name 2>/dev/null \
    | grep "^pod/deploy-dashboard-" | head -1 | sed 's#pod/##' || true)
  [ -n "$DEPPOD" ] && break
  sleep 2
done
if [ -z "$DEPPOD" ]; then
  echo "ERROR: no pod/deploy-dashboard-* found in namespace $NS" >&2
  exit 1
fi
echo "Deployment pod: $DEPPOD"

echo ""
echo "=== 4. Inject binary (tar stream; kubectl cp breaks on large files) ==="
# exec into the (still-running) initContainer, which shares the emptyDir with
# the main container. The init loop sees /app/deploy-dashboard and proceeds.
tar cf - -C /tmp deploy-dashboard-static \
  | kubectl -n "$NS" exec -i "$DEPPOD" -c wait-for-binary -- tar xf - -C /app/
kubectl -n "$NS" exec "$DEPPOD" -c wait-for-binary -- \
  sh -c 'mv /app/deploy-dashboard-static /app/deploy-dashboard && chmod +x /app/deploy-dashboard'
SHA_POD=$(kubectl -n "$NS" exec "$DEPPOD" -c wait-for-binary -- sha256sum /app/deploy-dashboard | awk '{print $1}')
echo "local sha: $SHA_LOCAL"
echo "pod   sha: $SHA_POD"
if [ "$SHA_LOCAL" != "$SHA_POD" ]; then
  echo "ERROR: binary checksum mismatch after injection" >&2
  exit 1
fi
echo "Binary verified in pod (sha matches)."

echo ""
echo "=== 5. Wait for readiness (probes) ==="
kubectl -n "$NS" wait --for=condition=Ready "pod/$DEPPOD" --timeout=120s

echo ""
echo "=== 6. Verify ==="
kubectl -n "$NS" logs "$DEPPOD" -c dashboard --tail=5

echo ""
echo "=== 7. Test (port-forward) ==="
kubectl -n "$NS" port-forward "svc/$DEPLOY" 8080:8080 &
PF_PID=$!
sleep 3

echo -n "Health:   "; curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8080/healthz
echo -n "Frontend: "; curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8080/
echo -n "API:      "; curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8080/api/deployments

kill $PF_PID 2>/dev/null || true

echo ""
echo "Deployment status:"
kubectl -n "$NS" get deploy "$DEPLOY"
echo ""
echo "Access via:"
echo "  - MetalLB:   http://172.16.1.90/ (from same L2 segment)"
echo "  - PortFwd:   kubectl -n $NS port-forward svc/$DEPLOY 8080:8080"
