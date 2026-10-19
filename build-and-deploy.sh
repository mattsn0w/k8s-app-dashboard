#!/bin/bash
# Build and deploy the deploy-dashboard to the k3s cluster.
#
# Usage:
#   ./build-and-deploy.sh [kubeconfig-path]
#
# This script:
#   1. Builds a static Go binary (the UI is embedded in the binary)
#   2. Creates the Pod
#   3. Copies the binary in
#   4. Verifies the deployment
set -euo pipefail

KUBECONFIG="${1:-$HOME/.kube/config}"
export KUBECONFIG
NS="apps"
POD_NAME="dashboard-runner"

echo "=== 1. Build static binary ==="
cd "$(dirname "$0")"
CGO_ENABLED=0 go build -ldflags="-s -w -extldflags=-static" -o /tmp/deploy-dashboard-static ./cmd/
echo "Binary: $(ls -lh /tmp/deploy-dashboard-static | awk '{print $5}')"

echo ""
echo "=== 2. Create Pod ==="
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $NS
  labels:
    app.kubernetes.io/name: deploy-dashboard
spec:
  serviceAccountName: deploy-dashboard
  containers:
  - name: dashboard
    image: debian:bookworm-slim
    command: ["sh", "-c"]
    args:
    - |
      echo "Waiting for binary..."
      while [ ! -f /app/deploy-dashboard ]; do
        sleep 5
      done
      chmod +x /app/deploy-dashboard
      echo "Starting deploy-dashboard..."
      exec /app/deploy-dashboard --listen=:8080 --namespace=$NS
    volumeMounts:
    - name: app
      mountPath: /app
    ports:
    - containerPort: 8080
      name: http
  volumes:
  - name: app
    emptyDir: {}
  restartPolicy: Always
EOF

echo ""
echo "=== 3. Wait for Pod ==="
kubectl wait --for=condition=Ready pod/$POD_NAME -n "$NS" --timeout=60s

echo ""
echo "=== 4. Copy binary ==="
# Pipe a tar stream through kubectl exec. `kubectl cp` frequently breaks on
# large binaries ("broken pipe"); a tar stream is reliable.
tar cf - /tmp/deploy-dashboard-static | kubectl exec -i -n "$NS" "$POD_NAME" -- tar xf - -C /tmp/
kubectl exec -n "$NS" "$POD_NAME" -- mv /tmp/deploy-dashboard-static /app/deploy-dashboard
kubectl exec -n "$NS" "$POD_NAME" -- chmod +x /app/deploy-dashboard

echo ""
echo "=== 5. Verify ==="
sleep 5
kubectl logs -n "$NS" "$POD_NAME" --tail=5

echo ""
echo "=== 6. Test ==="
# Port-forward and test
kubectl port-forward -n "$NS" "pod/$POD_NAME" 8080:8080 &
PF_PID=$!
sleep 3

echo -n "Health:   "
curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8080/healthz

echo -n "Frontend: "
curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8080/

echo -n "API:      "
curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8080/api/deployments

kill $PF_PID 2>/dev/null || true

echo ""
echo "✅ Deployed successfully!"
echo ""
echo "Access via:"
echo "  - NodePort:  http://<node-ip>:31143/ (Host: apps01.co.slakin.net)"
echo "  - MetalLB:   http://172.16.1.90/ (from same L2 segment)"
echo "  - PortFwd:   kubectl port-forward -n $NS pod/$POD_NAME 8080:8080"
