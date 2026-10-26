#!/usr/bin/env sh
# RBAC security test (SEC-10): Role-scoped access, no cluster-wide leak.
#
# Asserts, for the fixture ServiceAccount `sec-test-sa` (namespace
# `rbac-sec-test`, bound to a namespace-scoped Role):
#   1. It CAN perform the required verbs on namespaced resources in its namespace.
#   2. It is DENIED on cluster-scoped resources (and on the same namespaced
#      resources in OTHER namespaces).
#
# Uses `kubectl auth can-i --as=...` so no pod needs to run.
#
# Usage:  tests/k8s-role-sec/run.sh
# Needs:  kubectl + a kubeconfig that can create the `rbac-sec-test` namespace.
# Exits:  0 on all-pass, 1 on any failure.
set -u

NS=rbac-sec-test
SA=system:serviceaccount:${NS}:sec-test-sa
DIR=$(cd "$(dirname "$0")" && pwd)

# --- helpers ---------------------------------------------------------------
PASS=0
FAIL=0

# expect_ok VERB RESOURCE [NS]
#   SA should be ALLOWED.
expect_ok() {
  verb=$1; res=$2; ns=${3:-}
  if [ -n "$ns" ]; then
    out=$(kubectl auth can-i "$verb" "$res" -n "$ns" --as="$SA" 2>/dev/null)
  else
    out=$(kubectl auth can-i "$verb" "$res" --as="$SA" 2>/dev/null)
  fi
  [ "$out" = "yes" ] && { echo "  ok   : $verb $res ${ns:+-n $ns}"; PASS=$((PASS+1)); } \
                    || { echo "  FAIL : $verb $res ${ns:+-n $ns} (want yes, got '$out')"; FAIL=$((FAIL+1)); }
}

# expect_no VERB RESOURCE [NS]
#   SA should be DENIED.
expect_no() {
  verb=$1; res=$2; ns=${3:-}
  if [ -n "$ns" ]; then
    out=$(kubectl auth can-i "$verb" "$res" -n "$ns" --as="$SA" 2>/dev/null)
  else
    out=$(kubectl auth can-i "$verb" "$res" --as="$SA" 2>/dev/null)
  fi
  [ "$out" = "no" ] && { echo "  ok   : $verb $res ${ns:+-n $ns} (denied)"; PASS=$((PASS+1)); } \
                   || { echo "  FAIL : $verb $res ${ns:+-n $ns} (want no, got '$out')"; FAIL=$((FAIL+1)); }
}

# --- setup -----------------------------------------------------------------
echo "== RBAC security test (SEC-10) =="
if ! kubectl version --client >/dev/null 2>&1; then
  echo "ERROR: kubectl not found or not working." >&2; exit 1
fi
echo "-- applying fixture (namespace $NS) --"
if ! kubectl apply -f "$DIR/fixture.yaml" >/dev/null 2>&1; then
  echo "ERROR: could not apply $DIR/fixture.yaml (need rights to create ns $NS)." >&2
  exit 1
fi
# RBAC may take a moment to propagate.
sleep 2

# --- 1. Namespaced resources: required verbs are ALLOWED in the SA's ns ----
echo ""
echo "[1] Namespaced resources — required verbs allowed in '$NS':"
expect_ok  get    pods        "$NS"
expect_ok  list   pods        "$NS"
expect_ok  watch  pods        "$NS"
expect_ok  delete pods        "$NS"
expect_ok  get    pods/log    "$NS"
expect_ok  get    services    "$NS"
expect_ok  list   services    "$NS"
expect_ok  get    deployments "$NS"
expect_ok  create deployments "$NS"
expect_ok  update deployments "$NS"
expect_ok  patch  deployments "$NS"
expect_ok  delete deployments "$NS"
expect_ok  get    replicasets "$NS"
expect_ok  update replicasets "$NS"
expect_ok  patch  replicasets "$NS"
expect_ok  get    jobs        "$NS"
expect_ok  list   jobs        "$NS"

# --- 2. Cluster-scoped resources: DENIED -----------------------------------
# The SA has no ClusterRole, so it must be denied on cluster-scoped resources.
# (It reads no cluster data in this fixture; the real app reads only nodes/metrics
#  via a separate read-only ClusterRole, which is out of scope for this fixture.)
echo ""
echo "[2] Cluster-scoped resources — denied:"
expect_no  get    nodes
expect_no  list   nodes
expect_no  get    namespaces
expect_no  list   namespaces
expect_no  create namespaces
expect_no  get    clusterroles
expect_no  list   clusterroles
expect_no  get    clusterrolebindings
expect_no  delete nodes

# --- 3. Namespaced resources in OTHER namespaces: DENIED (no leak) ---------
# A Role only applies in its own namespace; confirm no cross-namespace access.
echo ""
echo "[3] Same namespaced resources in OTHER namespaces — denied (no leak):"
for other in default kube-system; do
  expect_no get    pods        "$other"
  expect_no delete pods        "$other"
  expect_no patch  deployments "$other"
done

# --- summary + cleanup -----------------------------------------------------
echo ""
echo "== Result: $PASS passed, $FAIL failed =="

# Tear down the fixture unless the user wants to inspect it.
if [ "${KEEP:-0}" != "1" ]; then
  echo "-- cleaning up (set KEEP=1 to leave the fixtures) --"
  kubectl delete namespace "$NS" --wait=false >/dev/null 2>&1
fi

[ "$FAIL" -eq 0 ] || exit 1
exit 0
