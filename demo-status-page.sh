#!/usr/bin/env bash
#
# demo-status-page.sh — demo the 503/50x "graceful status page" served by the
# Envoy Gateway direct-response filter.
#
# Architecture
#   The Devpod SPA is served by the `deploy-dashboard` Deployment under /app
#   (the backend runs with --base-path=/app). The gateway catch-all / returns a
#   small self-contained dark "Starting up" page via an HTTPRouteFilter
#   directResponse (body in the devpod-status-page ConfigMap, served by Envoy
#   itself — no backend required). That page polls /app/healthz every 3s; when
#   the backend is ready it reloads the real SPA at /app/ in the background.
#   So hitting the root URL while the backend is down shows the graceful page,
#   not a raw 503.
#
# What this script does
#   1. Saves the current replica count of deploy-dashboard.
#   2. Scales it to 0 (backend down -> Envoy has no healthy upstream).
#   3. Curls / (the root, now our direct-response status page) and /app/
#      (the SPA path, now a raw 503 while the backend is down).
#   4. Optionally holds the backend down (--hold N) so you can open a browser at
#      https://<host>/status and watch the loader poll /healthz.
#   5. Restores the backend to its original replica count and waits for Ready.
#
# Usage
#   ./demo-status-page.sh                 # quick cycle: down -> show -> restore
#   ./demo-status-page.sh --hold 30       # keep it down 30s so you can open a browser
#   URL=https://other.host/ ./demo-status-page.sh
#
set -euo pipefail

# --- configuration (override via env) ------------------------------------------
NS="${NS:-apps}"
DEPLOY="${DEPLOY:-deploy-dashboard}"
URL="${URL:-https://apps01.co.slakin.net}"
KUBECTL_ARGS="${KUBECTL_ARGS:---insecure-skip-tls-verify}"

k() { kubectl $KUBECTL_ARGS -n "$NS" "$@"; }

# --- args ----------------------------------------------------------------------
HOLD_SECS=0
while [ $# -gt 0 ]; do
  case "$1" in
    --hold)   HOLD_SECS="${2:-30}"; shift 2 ;;
    --hold=*) HOLD_SECS="${1#*=}"; shift ;;
    -h|--help) grep '^# ' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown arg: $1 (try --help)" >&2; exit 2 ;;
  esac
done

c() { # c <color> <text>
  local col="$1"; shift
  case "$col" in
    g) t=$'\033[32m';; y) t=$'\033[33m';; b) t=$'\033[34m';; r) t=$'\033[31m';; *) t='';;
  esac
  printf '%s%s%b' "$t" "$*" "${t:+\033[0m}"
}

wait_ready() {
  k wait --for=condition=Ready pod -l "app.kubernetes.io/name=$DEPLOY" --timeout=90s >/dev/null 2>&1
}

echo "== demo-status-page: $URL =="

# 1. current replicas
ORIG="$(k get deploy "$DEPLOY" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo 1)"
[ -z "$ORIG" ] && ORIG=1
c b "• Current replicas: $ORIG\n"

# Make sure the backend is up before starting (clean demo)
if [ "$ORIG" -lt 1 ]; then
  c y "• Was 0; restoring to 1 first...\n"
  k scale deploy "$DEPLOY" --replicas=1 >/dev/null
  wait_ready || { c r "✗ backend did not become ready\n"; exit 1; }
fi
c g "• Backend ready. Scaling to 0 to trigger the 50x direct response...\n"

# 2. take the backend down
k scale deploy "$DEPLOY" --replicas=0 >/dev/null
sleep 3   # let Envoy notice the upstream is gone

echo
c b "• [1] / (the root the user opens) — the graceful page Envoy serves via direct response:\n"
echo "----------------------------------------------------------------"
curl -sS -i -k "$URL/" 2>&1 | head -20
echo "----------------------------------------------------------------"

echo
c b "• [2] /app/ (the SPA path) while the backend is down — raw Envoy 503:\n"
echo "----------------------------------------------------------------"
curl -sS -i -k "$URL/app/" 2>&1 | head -15
echo "----------------------------------------------------------------"
c b "• (Open $URL/ in a browser while the backend is down to see the dark\n"
c b "   loading dialog. It polls /app/healthz every 3s; when I restore the\n"
c b "   backend below, it auto-reloads the SPA at /app/.)\n"

# 3. optional hold-down
if [ "$HOLD_SECS" -gt 0 ]; then
  c y "• Holding the backend DOWN for ${HOLD_SECS}s. Open in a browser NOW: $URL/\n"
  for ((s=HOLD_SECS; s>0; s--)); do
    printf '  down in %3ds...\r' "$s"; sleep 1
  done
  echo
fi

# 4. restore
echo
c b "• Restoring backend to $ORIG replica(s) and waiting for readiness...\n"
k scale deploy "$DEPLOY" --replicas="$ORIG" >/dev/null
# The fresh pod's emptyDir has no binary; the wait-for-binary init container
# blocks until one is injected. The init container is not immediately exec-able,
# so retry the injection until it works (or a local static build is absent).
if [ -f /tmp/deploy-dashboard-static ]; then
  POD="$(k get pods -o name 2>/dev/null | grep "^pod/$DEPLOY-" | head -1 | sed 's#pod/##')"
  if [ -n "${POD:-}" ]; then
    for i in 1 2 3 4 5 6 7 8; do
      if tar cf - -C /tmp deploy-dashboard-static \
           | k exec -i "$POD" -c wait-for-binary -- tar xf - -C /app/ 2>/dev/null; then
        k exec "$POD" -c wait-for-binary -- sh -c \
          'mv /app/deploy-dashboard-static /app/deploy-dashboard 2>/dev/null; chmod +x /app/deploy-dashboard' 2>/dev/null
        c b "• Injected static binary into the new pod.\n"
        break
      fi
      c y "• Waiting for the pod to accept the binary (attempt $i/8)...\n"
      sleep 3
    done
  fi
fi
if wait_ready; then
  c g "✓ Backend ready again — the status page will have auto-reloaded the SPA.\n"
else
  c r "✗ Backend not ready within 90s (may need a binary injection).\n"
fi

echo
echo "== final state =="
k get deploy "$DEPLOY"
