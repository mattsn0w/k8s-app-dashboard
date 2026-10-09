#!/bin/bash
# Build and deploy the deploy-dashboard to the k3s cluster.
#
# Usage:
#   ./build-and-deploy.sh [kubeconfig-path]
#
# This script:
#   1. Builds a static Go binary
#   2. Creates the ConfigMap with the base64-encoded HTML
#   3. Creates the Pod with the binary
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
echo "=== 2. Create ConfigMap with HTML ==="
HTML_B64=$(base64 -w0 web/index.html)
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: dashboard-html
  namespace: $NS
data:
  index.html.b64: "$HTML_B64"
EOF

echo ""
echo "=== 3. Create Pod ==="
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
    env:
    - name: EMBEDDED_HTML
      valueFrom:
        configMapKeyRef:
          name: dashboard-html
          key: index.html.b64
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
echo "=== 4. Wait for Pod ==="
kubectl wait --for=condition=Ready pod/$POD_NAME -n "$NS" --timeout=60s

echo ""
echo "=== 5. Copy binary ==="
kubectl cp /tmp/deploy-dashboard-static "$NS/$POD_NAME:/app/deploy-dashboard"

echo ""
echo "=== 6. Verify ==="
sleep 5
kubectl logs -n "$NS" "$POD_NAME" --tail=5

echo ""
echo "=== 7. Test ==="
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
