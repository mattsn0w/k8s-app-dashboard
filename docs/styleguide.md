# Devpod Style Guide

This document is the single source of truth for the Devpod dashboard's visual
identity: colors, themes, and icons. **Update this file whenever you change
styling** so the UI and its documentation never drift apart.

## Frontend design system: Primer (GitHub)

**The Devpod frontend is built exclusively on GitHub's [Primer design system]
(https://primer.style).** Primer is the mandatory source for **both** the UI
components **and** the icons. Do not introduce ad-hoc, hand-rolled, or third-party
UI styles or icons outside of Primer.

Concretely:

- **UI / components** — use Primer CSS (loaded from the Primer CDN in
  `web/index.html`) and its component classes (`.btn`, `.table`, `.tabnav`, forms,
  modals, etc.). Map Primer's semantic tokens (`--bgColor-*`, `--fgColor-*`,
  `--borderColor-*`, `--shadow-*`) to the Catppuccin palette per theme instead of
  defining new, parallel design tokens. New UI should reuse an existing Primer
  component/pattern before adding custom CSS.
- **Icons** — use **Octicons** (Primer's icon library) only. Every icon is an
  inline SVG `<symbol>` in the sprite at the top of `web/index.html`, and each must
  be copied **verbatim** from the official `primer/octicons` repo
  (`icons/<name>-16.svg`) and referenced with `<use href="#octicon-..."/>`.
  **Never hand-draw an icon path** — always verify it against the official SVG
  (see the icon-verification note in `issues/`).
- **Sizing / spacing / typography** — follow Primer conventions (icon `16px`,
  Primer spacing and type scale) so the app looks native to the GitHub design
  language.

Catppuccin (Mocha for dark, Latte for light) is used only to **theme** Primer —
the *shape* of the UI and the *icons* come from Primer; Catppuccin supplies the
*colors*.

[primer design system]: https://primer.style

## Themes

The app uses two Catppuccin flavors, selected via the `data-color-mode` attribute
on `<html>`:

| Theme | Fl | `data-color-mode` |
|-------|------|-------------------|
| Dark | **Catppuccin Mocha** | `dark` |
| Light | **Catppuccin Latte** | `light` |

Colors are defined once as CSS custom properties on each `[data-color-mode]` block
in `web/index.html`. Prefer referencing those variables (never hardcode a hex in a
rule) so a future palette change is a one-line edit.

## Catppuccin palette tokens used

### Dark — Mocha
| Token | Hex | Used for |
|-------|-----|----------|
| crust | `#11111b` | App header background |
| mantle | `#181825` | Tab list background |
| base | `#1e1e2e` | Active tab background |
| blue | `#89b4fa` | Active tab accent (foreground), starting/in-progress status |
| teal | `#94e2d5` | good/running status |
| overlay2 | `#9399b2` | stopped status |
| red | `#f38ba8` | failed/error status, delete action |
| rosewater | `#f5e0dc` | view-logs action |
| peach | `#fab387` | terminate-pods action |

### Light — Latte
| Token | Hex | Used for |
|-------|-----|----------|
| crust | `#dce0e8` | App header background |
| mantle | `#cdd1da` | Tab list background |
| base | `#eff1f5` | Active tab background |
| blue | `#1e66f5` | Active tab accent (foreground), starting/in-progress status |
| teal | `#179299` | good/running status |
| overlay2 | `#6c6f85` | stopped status |
| red | `#d20f39` | failed/error status, delete action |
| rosewater | `#b4637a` | view-logs action |
| peach | `#fe640b` | terminate-pods action |

## CSS variables

Defined on both `[data-color-mode="dark"]` (mocha) and `[data-color-mode="light"]`
(latte) in `web/index.html`:

| Variable | Role |
|----------|------|
| `--header-bg` | App header background |
| `--tabs-bg` | Tab list background |
| `--tab-active-bg` | Active tab background |
| `--tab-active-accent` | Active tab foreground accent |
| `--status-good` | Running / good / succeeded |
| `--status-stopped` | Stopped / not running |
| `--status-starting` | Starting / in-progress / pending / running |
| `--status-bad` | Failed / error / crashloopbackoff |
| `--action-logs` | "View logs" button |
| `--action-terminate` | "Terminate pods" button |
| `--action-delete` | "Delete deployment" button |

General UI tokens (Primer-mapped) remain on the same blocks: `--bgColor-*`,
`--fgColor-*`, `--borderColor-*`, `--shadow-*`.

## Status colors

| State | Dark (Mocha) | Light (Latte) |
|-------|--------------|---------------|
| Running / good / succeeded | `#94e2d5` (teal) | `#179299` (teal) |
| Stopped | `#9399b2` (overlay2) | `#6c6f85` (overlay2) |
| Starting / in-progress | `#89b4fa` (blue) | `#1e66f5` (blue) |
| Failed / error | `#f38ba8` (red) | `#d20f39` (red) |

Status badges are translucent pills: `background: color-mix(in srgb, <var> ~20%,
transparent)`, with text and 1px border in the solid `<var>`.

## Icons (Octicons — Primer)

Icons come **only** from **Octicons** (Primer's icon library) — see the
[Primer policy](#frontend-design-system-primer-github) above. No hand-drawn or
third-party icons.

All icons are inline SVG `<symbol>`s in the sprite at the top of `web/index.html`.
Add new icons to that sprite by copying the **official 16px Octicon** SVG verbatim
from the `primer/octicons` repo (`icons/<name>-16.svg`); reference them with
`<use href="#octicon-..."/>`. **Always verify an icon's path against the official
SVG** (do not hand-draw paths) — see the icon-verification note in `issues/`.

| Element | Octicon |
|---------|---------|
| App header | `rocket` |
| Deployments tab | `cloud` |
| Jobs tab | `beaker` |
| Cluster Nodes tab | `pulse` |
| Status badge — running/good | `cloud` |
| Status badge — stopped | `cloud-offline` |
| Status badge — starting/in-progress | `webhook` |
| Action — View logs | `log` |
| Action — Terminate pods | `issue-reopened` |
| Action — Delete deployment | `crosshairs` |
| User settings | `gear` |
| Menu — SSH Key | `key` |
| Menu — Access Tokens | `lock` |
| Menu — Help/Docs | `info` |
| Menu — Log out | `sign-out` |
| Menu — Theme (dark) | `moon` |
| Menu — Theme (light) | `sun` |

## Workload icon (per-deployment)

A deployment may declare a single icon for itself via the
**`workload-icon`** annotation, whose value is **strictly a Primer Octicon name**
(e.g. `terminal`, `server`, `vscode`). The icon is rendered inline to the left of
the deployment name in the Deployments tab.

- **Absent** (or set to an invalid/non-Octicon value) → **no icon** is shown.
- **Present + valid** → the matching Octicon from the sprite is shown. The name
  is validated against the strict Octicon-name grammar (lowercase alphanumerics
  + single hyphens, no leading/trailing/consecutive hyphens) so a bad annotation
  never produces a broken `<use href>`.

Because the UI only ships the icons present in the sprite, a `workload-icon` value
that is a valid Octicon name but **not** embedded in `web/index.html` will render
blank (an empty `<use>`). To use a new icon, add its official 16px Octicon symbol
to the sprite (see the Primer policy above).

The Go backend exposes it as `workloadIcon` (omitted when absent) in
`GET /api/deployments`; the frontend renders it as
`<svg class="octicon workload-icon"><use href="#octicon-<name>"/></svg>`.

Demo workloads (in `manifests/04-demo-workloads.yaml`):

| Deployment  | `workload-url`  | `workload-icon` |
|-------------|-----------------|-----------------|
| echo-server | `/apps/echo`    | `server`        |
| ttyd-terminal | `/apps/terminal` | `terminal`     |
| vscode-web  | `apps/vscode/`  | `vscode`        |

## Action button colors

| Action | Dark (Mocha) | Light (Latte) |
|--------|--------------|---------------|
| View logs | `#f5e0dc` (rosewater) | `#b4637a` (rosewater) |
| Terminate pods | `#fab387` (peach) | `#fe640b` (peach) |
| Delete deployment | `#f38ba8` (red) | `#d20f39` (red) |

Action buttons are `.btn-icon` with a modifier class (`.logs`, `.warning`,
`.danger`); hover fills the button with the color and switches the icon to the
panel background.

## Change process

1. Prefer a **Primer** component/icon for any new UI. Edit the CSS variables / rules
   in `web/index.html` (reusing Primer classes/tokens; theme Catppuccin per theme).
2. Update the corresponding table(s) in this file.
3. Add the referenced Octicon to the sprite **from the official `primer/octicons`
   SVG** if a new icon was used; verify its path matches the official icon.
4. Rebuild the static binary (`CGO_ENABLED=0 go build …`), hot-swap it into the
   `deploy-dashboard` pod, and verify in the browser (both themes).
5. Reference the change from the relevant issue in `issues/`.

## Change log

| Date | Change | Issue(s) |
|------|--------|----------|
| 2026-10-25 | Dark status colors set to exact **mocha** tokens; light status colors set to exact **latte** tokens; added `--header-bg`/`--tabs-bg`/`--tab-active-*`/`--action-*` variables; header→`rocket`, Deployments tab→`cloud`, status icons→`cloud`/`cloud-offline`/`webhook`, action icons→`log`/`issue-reopened`/`crosshairs`. | APP-17, APP-18 |
| 2026-10-25 | Added the **Primer-only** frontend policy: UI components and icons must strictly use the Primer library (Primer CSS + Octicons). Catppuccin is used only to theme Primer. All hand-drawn icons replaced with verbatim official 16px Octicons. | style |
| 2026-10-25 | Added the **`workload-icon`** per-deployment annotation (strict Primer Octicon name; absent → no icon). Added `terminal`, `server`, `vscode` Octicons to the sprite; `echo-server`→`server`, `ttyd-terminal`→`terminal`, new `vscode-web`→`vscode`. Relaxed `isValidWorkloadURL` to accept trailing-slash relative subpaths (e.g. `vscode/`). | workload-icon |
