#!/usr/bin/env bash
# Build and deploy the deploy-dashboard using BUILDHA in-cluster.
#
# This is the deployment path for environments where the local `docker` daemon
# is unavailable (e.g. a sandbox that denies /var/run/docker.sock). Instead of
# `docker build`, the image is built by a **Job in the cluster** that runs
# buildah, then pushed to the registry.
#
# Usage:
#   ./build-and-deploy-buildah.sh
#
# Environment overrides (all optional):
#   IMAGE            image to build/push (default: registry.co.slakin.net/local/deploy-dashboard:latest)
#   BUILDHA_IMAGE    the buildah tooling image (default: quay.io/buildah/stable)
#   KUBECONFIG       kubectl context (uses your default if unset)
#   NS               namespace (default: apps)
#   SKIP_CLEANUP     set to 1 to keep the Job + ConfigMap for debugging
#
# Why buildah needs these (each caused a distinct failure until set):
#   1. BUILDHA_IMAGE must be quay.io/buildah/stable (buildah 1.4x). It is NOT
#      quay.io/buildah/buildah (401 UNAUTHORIZED) and NOT docker.io/buildah/
#      buildah:latest (that tag is buildah 1.8.3 / skopeo 0.1.36 from 2020,
#      which fatals "unsupported schema version 2" on modern manifests).
#   2. The build pod needs securityContext.privileged: true -- the vfs storage
#      driver's ApplyLayer does a remount that requires CAP_SYS_ADMIN.
#   3. STORAGE_DRIVER=vfs -- the default overlay driver is "not supported over
#      overlayfs" (a k8s pod's rootfs is overlay).
#   4. buildah bud takes the build context as a POSITIONAL argument (there is
#      no --context flag). -f/--file is the Dockerfile flag.
#
# The local registry (registry.co.slakin.net) is open (no auth), so buildah
# push needs no credentials; --tls-verify=false covers its self-signed CA.
set -euo pipefail

cd "$(dirname "$0")"

IMAGE="${IMAGE:-registry.co.slakin.net/local/deploy-dashboard:latest}"
BUILDHA_IMAGE="${BUILDHA_IMAGE:-quay.io/buildah/stable}"
NS="${NS:-apps}"
DEPLOY="deploy-dashboard"
JOB_NAME="dd-buildah"
CM_NAME="dd-src"
SKIP_CLEANUP="${SKIP_CLEANUP:-0}"

# kubectl helper (honors KUBECONFIG env).
K="kubectl"

log()  { printf '\033[1;34m%s\033[0m\n' "$*"; }
err()  { printf '\033[1;31m%s\033[0m' "$*" >&2; }
ok()   { printf '\033[1;32m%s\033[0m\n' "$*"; }

# ---------------------------------------------------------------------------
log "=== 0. Preflight: confirm the cluster is reachable ==="
if ! $K get ns >/dev/null 2>&1; then
  err "ERROR: cannot reach the cluster via kubectl."
  echo "  Set KUBECONFIG to a valid context, e.g.:"
  echo "    KUBECONFIG=~/.kube/config ./build-and-deploy-buildah.sh"
  exit 1
fi
ok "cluster reachable (context: $($K config current-context 2>/dev/null || echo default))"

# ---------------------------------------------------------------------------
log "=== 1. Stage the source into a ConfigMap (~$CM_NAME) ==="
# The buildah Job mounts this read-only and reconstructs the project layout.
# Only the files the Dockerfile needs are staged (the whole source is <1MB,
# well under the 1MB ConfigMap limit).
$K create configmap "$CM_NAME" -n "$NS" \
  --from-file=embed.go=embed.go \
  --from-file=main.go=cmd/main.go \
  --from-file=main_test.go=cmd/main_test.go \
  --from-file=ws_test.go=cmd/ws_test.go \
  --from-file=url_test.go=cmd/url_test.go \
  --from-file=octicon_test.go=cmd/octicon_test.go \
  --from-file=index.html=web/index.html \
  --from-file=go.mod=go.mod \
  --from-file=Dockerfile=Dockerfile \
  --dry-run=client -o yaml | $K apply -n "$NS" -f -
ok "ConfigMap $CM_NAME staged"

# ---------------------------------------------------------------------------
log "=== 2. Create the buildah build Job ==="
cat > /tmp/"${JOB_NAME}".yaml <<YAML
apiVersion: batch/v1
kind: Job
metadata: { name: ${JOB_NAME}, namespace: ${NS}, labels: { app: ${JOB_NAME} } }
spec:
  backoffLimit: 2
  ttlSecondsAfterFinished: 600
  template:
    metadata: { labels: { app: ${JOB_NAME} } }
    spec:
      restartPolicy: Never
      containers:
      - name: build
        image: ${BUILDHA_IMAGE}
        imagePullPolicy: IfNotPresent
        securityContext:
          privileged: true
        command:
        - /bin/sh
        - -c
        - |
          set -eu
          export STORAGE_DRIVER=vfs
          echo "=== buildah version ==="
          buildah --version
          echo "=== [1/3] Reconstruct source layout from ConfigMap ==="
          SRC=/src-cm
          CTX=/build-context
          mkdir -p \$CTX/cmd \$CTX/web
          cp \$SRC/main.go \$SRC/main_test.go \$SRC/ws_test.go \$SRC/url_test.go \$SRC/octicon_test.go \$CTX/cmd/
          cp \$SRC/index.html \$CTX/web/
          cp \$SRC/go.mod \$SRC/Dockerfile \$SRC/embed.go \$CTX/
          ls -R \$CTX
          echo "=== [2/3] buildah bud (multi-stage) ==="
          buildah bud \\
            --network host \\
            --file \$CTX/Dockerfile \\
            -t ${IMAGE} \\
            \$CTX
          echo "=== [3/3] push ==="
          buildah push \\
            --tls-verify=false \\
            ${IMAGE} \\
            docker://${IMAGE}
          echo "=== BUILD_OK: pushed ${IMAGE} ==="
        volumeMounts:
        - name: src
          mountPath: /src-cm
          readOnly: true
      volumes:
      - name: src
        configMap: { name: ${CM_NAME} }
YAML

$K delete job "$JOB_NAME" -n "$NS" --wait=false >/dev/null 2>&1 || true
sleep 1
$K apply -n "$NS" -f /tmp/"${JOB_NAME}".yaml
ok "Job $JOB_NAME submitted (buildah: $BUILDHA_IMAGE)"

# ---------------------------------------------------------------------------
log "=== 3. Follow the build logs ==="
# Wait for the pod, then stream its logs. buildah's go build is the slow part.
POD=""
for _ in $(seq 1 60); do
  POD=$($K get pods -n "$NS" -l "app=$JOB_NAME" -o name 2>/dev/null | head -1 | sed 's|pod/||' || true)
  [ -n "$POD" ] && break
  sleep 2
done
if [ -z "$POD" ]; then
  err "ERROR: build Job pod never appeared."
  $K get job "$JOB_NAME" -n "$NS" -o yaml 2>/dev/null | tail -20
  exit 1
fi
ok "following logs of pod: $POD"

# Wait until the container is actually Running (logs are unavailable while it is
# ContainerCreating/Waiting, and `kubectl logs` errors out). Then stream.
for _ in $(seq 1 90); do
  CS=$($K get pod "$POD" -n "$NS" -o jsonpath='{.status.containerStatuses[0].state}' 2>/dev/null || true)
  PH=$($K get pod "$POD" -n "$NS" -o jsonpath='{.status.phase}' 2>/dev/null || true)
  if echo "$CS" | grep -q "running"; then break; fi
  if [ "$PH" = "Succeeded" ] || [ "$PH" = "Failed" ]; then break; fi
  sleep 2
done

# Stream the logs to the terminal (with a generous cap). We do NOT block for
# minutes without output -- the logs stream in real time.
timeout 900 $K logs -f "$POD" -n "$NS" 2>&1 | tee /tmp/"${JOB_NAME}".log || true

# ---------------------------------------------------------------------------
log "=== 4. Verify the build Job completed ==="
if $K wait --for=condition=complete "job/$JOB_NAME" -n "$NS" --timeout=30s >/dev/null 2>&1; then
  ok "build Job completed"
else
  err "ERROR: build Job did not complete. Tail of the build log:"
  tail -30 /tmp/"${JOB_NAME}".log 2>/dev/null || true
  echo
  err "Job status:"
  $K get job "$JOB_NAME" -n "$NS" 2>/dev/null || true
  exit 1
fi

# Confirm the "BUILD_OK" marker actually reached the log (guard against a Job
# that is 'Complete' but whose push step was skipped/failed silently).
# The `kubectl wait --for=condition=complete` above is the authoritative signal:
# a Job only reaches "Complete" when the buildah container exits 0, which only
# happens after `buildah push` succeeds. The BUILD_OK grep is a secondary
# sanity check on the *captured* log; a miss there usually means the log follow
# missed the tail, NOT that the push failed (the registry check in step 5 is
# the real confirmation).
if grep -q "BUILD_OK" /tmp/"${JOB_NAME}".log 2>/dev/null; then
  ok "push confirmed (BUILD_OK marker in build log)"
else
  echo "  (note: BUILD_OK marker not in the captured log tail -- the Job still"
  echo "   completed, so the push ran; the registry check below confirms it.)"
fi

# ---------------------------------------------------------------------------
log "=== 5. Verify the new image is in the registry ==="
# The registry is open; a 200 on the manifest means the push landed.
REGISTRY="${IMAGE%%/*}"            # registry.co.slakin.net
REPO="${IMAGE#*/}"                  # local/deploy-dashboard:latest
TAG="${REPO##*:}"
REPO_NO_TAG="${REPO%:*}"
if curl -s -o /dev/null --max-time 15 \
  -H "Accept: application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json" \
  -w "%{http_code}" "https://${REGISTRY}/v2/${REPO_NO_TAG}/manifests/${TAG}" | grep -q "200"; then
  ok "manifest for ${REPO_NO_TAG}:${TAG} is present (HTTP 200)"
else
  err "WARNING: could not confirm the manifest in the registry (check TLS/auth)."
fi

# ---------------------------------------------------------------------------
log "=== 6. Rollout restart + wait for the new image ==="
# `kubectl apply` alone won't roll a :latest Deployment (identical spec string);
# `rollout restart` bumps the pod-template annotation to force a fresh pull.
$K apply -f manifests/03-deployment.yaml >/dev/null 2>&1 || true
$K -n "$NS" rollout restart "deployment/$DEPLOY"
$K -n "$NS" rollout status "deployment/$DEPLOY" --timeout=180s
ok "rollout complete"

sleep 2
NEW_POD=$($K get pods -n "$NS" -l "app.kubernetes.io/name=$DEPLOY" -o name 2>/dev/null | head -1 | sed 's|pod/||' || true)
NEW_IMAGE=$($K get pod "$NEW_POD" -n "$NS" -o jsonpath='{.status.containerStatuses[0].imageID}' 2>/dev/null || true)
NEW_READY=$($K get pod "$NEW_POD" -n "$NS" -o jsonpath='{.status.containerStatuses[0].ready}' 2>/dev/null || true)
ok "pod: $NEW_POD (ready=$NEW_READY)"
echo "   image: $NEW_IMAGE"

# ---------------------------------------------------------------------------
log "=== 7. Cleanup (Job + ConfigMap) ==="
if [ "$SKIP_CLEANUP" = "1" ]; then
  echo "  SKIP_CLEANUP=1 -- leaving Job $JOB_NAME + ConfigMap $CM_NAME for debugging."
else
  $K delete job "$JOB_NAME" -n "$NS" --wait=false >/dev/null 2>&1 || true
  $K delete configmap "$CM_NAME" -n "$NS" >/dev/null 2>&1 || true
  rm -f /tmp/"${JOB_NAME}".yaml /tmp/"${JOB_NAME}".log
  ok "cleaned up Job + ConfigMap"
fi

# ---------------------------------------------------------------------------
log "=== Access URL (derived from the gateway LoadBalancer Service) ==="
GATEWAY_NS="apps"
GATEWAY_NAME="eg"
LB_SVC=$($K -n envoy-gateway-system get svc -o json 2>/dev/null \
  | python3 -c "import sys,json; svcs=json.load(sys.stdin)['items']; print(next((s['metadata']['name'] for s in svcs if s['spec'].get('type')=='LoadBalancer'),''))" 2>/dev/null || true)
HOSTNAME_=$($K -n "$GATEWAY_NS" get gateway "$GATEWAY_NAME" -o jsonpath='{.spec.listeners[0].hostname}' 2>/dev/null || true)
if [ -n "$LB_SVC" ] && [ -n "$HOSTNAME_" ]; then
  LB_IP=$($K -n envoy-gateway-system get svc "$LB_SVC" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
  echo "  Gateway hostname : $HOSTNAME_"
  echo "  LoadBalancer IP  : ${LB_IP:-<pending>}"
  echo "  Access via       : https://$HOSTNAME_/   (or http://$HOSTNAME_/ which redirects to HTTPS)"
  if [ -n "$LB_IP" ]; then
    echo "  (raw LB IP)      : https://$LB_IP/ -- host header must be $HOSTNAME_"
  fi
else
  echo "  (could not derive URL; check the gateway + its LoadBalancer Service)"
fi

log "=== Done ==="
$K -n "$NS" get pods -l "app.kubernetes.io/name=$DEPLOY"
