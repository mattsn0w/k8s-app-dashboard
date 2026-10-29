# APP-19: Switching pods in the log modal disconnects the stream

- **Type:** App (regression introduced by UX-03)
- **Summary:** In the log modal, toggling the Pod (or Container) selector to
  another pod disconnects the log stream instead of cleanly re-streaming the new
  pod. The old socket's `onclose` fires *after* the new socket is opened and,
  because the modal is still open, schedules a 3s "reconnect" timer that then
  kills the *new* (correctly-streaming) socket — producing visible
  "Disconnected, reconnecting in 3s…" churn.
- **Severity:** High
- **Effort:** Low
- **Status:** ✅ **FIXED + DEPLOYED + BROWSER-VERIFIED** (2026-10-29)

## Root cause
`openLogModalFor()` closes the old socket and opens a new one on every
pod/container switch. But the **old socket's `onclose` handler is still
attached** and fires asynchronously after `.close()`. That handler only checks
`modal.classList.contains('open')` (still true during a switch) — so it runs
`setTimeout(restartLogStream, 3000)`. Three seconds later that **stale** timer
fires `restartLogStream()`, which closes the *new* socket and opens yet another.
Net effect: every pod switch caused a disconnect/reconnect ~3s later. The same
latent bug existed in the WS `onmessage` `end`-reconnect path.

## Fix (`web/index.html`)
Introduced a **stream-generation token** so a superseded socket cannot act:
- `let logStreamToken = 0;` — module state.
- `openLogModalFor()`: at the top, `const token = ++logStreamToken;` and
  `const isCurrent = () => token === logStreamToken;`. **Every** handler
  (`onopen`, `onmessage`, `onerror`, `onclose`) and **both** reconnect timers
  now guard with `if (!isCurrent()) return;` (and the timers also re-check
  `isCurrent()` when they fire). So a socket that was superseded by a newer
  open (pod/container switch) or by a modal close sees a stale token and stays
  inert — it cannot schedule or fire a phantom reconnect.
- `closeLogModal()`: also does `logStreamToken++` so a closing socket's
  `onclose`/`onmessage` and any already-pending reconnect timer are invalidated.

## Verification
- **10 new regression tests** in `web/test.js` (`log stream token guard`),
  modeling the open/close/reconnect lifecycle with the token guard:
  - superseded socket's `onclose` schedules **no** reconnect;
  - current socket's `onclose` (modal open) **does** schedule one, and it fires;
  - closing the modal cancels a pending reconnect (token bump);
  - rapid A→B→C switches produce no phantom reconnects; only the last socket is
    current.
- **`node web/test.js`: 131 passed, 0 failed.**
- **Live browser proof** on `echo-server` (2 replicas):
  - Switch pod once, wait 10s (past the 3s bug window): status =
    `Streaming <new-pod>`, title updated, token advanced by exactly 1 (2→3),
    200 lines streaming. **Not** "Disconnected, reconnecting…".
  - 3 rapid pod toggles + settle: still `Streaming <final-pod>`, token = 6
    (one increment per intentional switch, no extra increments from phantom
    reconnects).

## Deploy
Built with **buildah** (see "In-cluster buildah build" below), pushed
`registry.co.slakin.net/local/deploy-dashboard:latest@sha256:619251ce…`,
`rollout restart`. New pod Ready. Served HTML contains `logStreamToken` /
`isCurrent`.

## In-cluster buildah build (deploy method, 2026-10-29)
The local `docker` daemon is denied in the sandbox, so the image is built
**in-cluster** with buildah. After the cluster's registry settings were updated
(egress opened), `quay.io/buildah/stable` (buildah 1.43.5) is pullable and works.

Requirements that had to be satisfied (each caused a distinct failure until set):
1. **Image ref is `quay.io/buildah/stable`** — NOT `quay.io/buildah/buildah`
   (401) and NOT `docker.io/buildah/buildah:latest` (that tag is buildah 1.8.3 /
   skopeo 0.1.36 from 2020, which fatals `unsupported schema version 2` on this
   cluster's modern manifests).
2. **`securityContext.privileged: true`** on the build pod — the `vfs` storage
   driver's `ApplyLayer` does a `remount` that needs `CAP_SYS_ADMIN`.
3. **`STORAGE_DRIVER=vfs`** — the default `overlay` driver is "not supported
   over overlayfs" (k8s pod rootfs is overlay).
4. **Buildah `bud` context is a positional argument** — there is no `--context`
   flag (`buildah bud … $CTX`). `-f/--file` is the Dockerfile flag.

Job shape: stage the source (~172KB: `embed.go`, `cmd/*.go`, `web/index.html`,
`go.mod`, `Dockerfile`) into a **ConfigMap**, then a Job running
`quay.io/buildah/stable` (privileged, `STORAGE_DRIVER=vfs`) that reconstructs
the layout, runs
`buildah bud --network host --file $CTX/Dockerfile -t <image> $CTX`, then
`buildah push --tls-verify=false <image> docker://<image>` (the local registry
is open / no auth). `kubectl rollout restart deployment/deploy-dashboard`.

Note: `docker.io/moby/buildkit:latest` also works here (used for earlier deploys)
if buildah is unavailable; buildah's build was ~26s vs BuildKit's ~6min.
