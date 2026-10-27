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
