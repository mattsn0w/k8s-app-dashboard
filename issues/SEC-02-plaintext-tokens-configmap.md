# SEC-02: Secrets stored in a plaintext ConfigMap

- **Type:** Security
- **Summary:** `handleTokens`/`putUserSettings` write user-supplied "access tokens" (and SSH keys) into the `user-settings` **ConfigMap** in `apps`. ConfigMaps are not encrypted at rest by default and are readable by anyone holding `get` on ConfigMaps in the namespace.
- **Severity:** High
- **Effort:** Medium
- **Status:** ✅ FIXED

## Resolution
- Backend `getUserSettings`/`upsertUserSettings`/`putUserSettings` now read/write the
  `user-settings` **Secret** (not a ConfigMap). Writes use `stringData` (plain values;
  the API server base64-encodes); reads decode `data` and let `stringData` override.
  The API surface (`/api/user-settings`, `/api/tokens`) and the frontend are unchanged.
- `manifests/02-rbac.yaml`: added a namespaced `Role` + `RoleBinding`
  (`deploy-dashboard-user-settings`) granting `get/update/patch` on the single named
  Secret `user-settings` (least privilege — no list over all secrets, no create).
- Migrated existing values into the Secret and deleted the plaintext ConfigMap.
- Verified in-cluster: `auth can-i` allows only get/update/patch on `secrets/user-settings`;
  denies list secrets, get on other secrets, and all configmap verbs. Read + write round-trip
  confirmed against the live Secret.

---

## Why this issue should be addressed / ranked
Tokens are, by the feature's own intent, credentials. Storing credentials in the wrong K8s primitive defeats the purpose of a secret and makes them trivially exfiltrable via `kubectl get cm -o yaml`. Ranked just below authentication because it only becomes exploitable once someone (a human, or a client per SEC-01) can read the namespace.

## Justification for the severity level
High. Credential-at-rest leakage is a standard audit finding. ConfigMaps appear in `kubectl get all -o wide`-style dumps, CI logs, and etcd snapshots of a leaked cluster. A Secret (ideally with encryption-at-rest) or an external vault is the correct primitive.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing; independent. Should be fixed before any real user pastes credentials. Pairs naturally with SEC-01 so the read path for the secret is also gated.

## Justification for the effort level
Medium. The change is localized: switch backend read/write to a `Secret` (base64 the values, use the secrets API), update the Role/RoleBinding in `manifests/02-rbac.yaml` from `configmaps` to `secrets`, and migrate existing data. The UI is unchanged. Cost is touching RBAC and the storage code paths, not new design.
