# SEC-03: Dashboard RBAC grants full ConfigMap CRUD (create)

- **Type:** Security
- **Summary:** `manifests/02-rbac.yaml` grants the dashboard ServiceAccount `get/list/watch/create/update/patch` on `configmaps` in `apps`. That lets the app **create** any ConfigMap name in the namespace, not just `user-settings`, widening its blast radius beyond what the feature needs.
- **Severity:** Medium
- **Effort:** Low
- **Status:** ✅ FIXED

## Resolution
- The over-broad configmap grants lived in two live-only namespaced Roles
  (`deploy-dashboard` and `deploy-dashboard-configmaps`), each granting
  `get/list/watch/create/update/patch` on `configmaps`. Both Roles + their RoleBindings
  were deleted; the committed `manifests/02-rbac.yaml` ClusterRole never had a configmap rule.
- Replaced with a least-privilege, name-scoped grant: a namespaced `Role` + `RoleBinding`
  (`deploy-dashboard-user-settings`) allowing `get/update/patch` on the single Secret
  `user-settings` (see SEC-02). The app can no longer create/list any ConfigMap, nor read
  any other Secret.
- Verified with `auth can-i`: create/list/get configmaps = denied; get/update/patch on
  `secrets/user-settings` = allowed.

---

## Why this issue should be addressed / ranked
Least-privilege is the point of a dedicated ServiceAccount. The app only ever reads/writes one known ConfigMap (`user-settings`), so granting namespace-wide `create` is unnecessary and would let a compromised or buggy handler write arbitrary ConfigMaps (which other apps may consume). Cheap to tighten, so it is a high-value, low-cost item.

## Justification for the severity level
Medium. It is an over-broad grant rather than an active leak. Exploitable only if the app itself is compromised (which requires SEC-01 first) and then abused to plant ConfigMaps. Still worth fixing because it is a one-line manifest change.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing. If SEC-02 (tokens move to Secrets) lands, this file changes again to the secrets grant, so do the two together.

## Justification for the effort level
Low. Edit the verb list in one Role and, if desired, scope the ConfigMap resource to a single name. No code change, no runtime risk beyond re-applying RBAC.
