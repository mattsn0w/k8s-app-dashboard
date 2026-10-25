# APP-16: Add a `logout` item and link to settings menu.

- **Type:** Feature
- **Summary:** Create an a logout menu item and link to the relative subpath `/logout` , e.g. `/u/user1/logout`
- **Severity:** Low
- **Effort:** Medium

## Dependencies
- **Depends on:** OIDC auth.
- **Enables:** clears the browser of JWT and tells OIDC service to revoke the signed JWT.


---
- **Status:** DONE

## Implementation
- Added a **Log out** item to the user settings dropdown menu (`web/index.html`),
  rendered as a link with the `octicon-sign-out` icon (symbol added to the sprite).
- The href is computed at load in `initLogoutLink()`: `<app-base-path>/logout`,
  where the base is the document's directory (path up to and including the last `/`).
  - Served at `/`        → `/logout`
  - Served at `/u/user1/` → `/u/user1/logout`
- No backend change required: the gateway/OIDC provider handles the `/logout` route
  (revoke + clear JWT). The frontend just links there.
- Rebuilt the static binary with the new embedded HTML and hot-swapped it into the
  running `deploy-dashboard` pod (verified `logoutLink` present, pod Ready).
- Verified in the browser: menu item renders, href resolves to the base path + `logout`.
