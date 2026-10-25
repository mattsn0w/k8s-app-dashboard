# APP-17: Soften status-badge colors

- **Type:** UX / Aesthetic
- **Summary:** The `success`/`running` status badges use a very bright green that
  is harsh on both light and dark themes. Replace the status colors with the softer
  Catppuccin palette values below (applied to both light and dark modes).
- **Severity:** Low (aesthetic, non-blocker)
- **Effort:** Trivial

## Requested colors
| State | Color | Hex |
|-------|-------|-----|
| Running / Good / Succeeded | teal-green | `#8bd5ca` |
| Stopped (0 containers) | muted gray-blue | `#cad3f5` |
| Starting / Pending | blue | `#7dc4e4` |
| Failed / Error / CrashLoopBackOff | red | `#ed8796` |

## Scope
- `.status-badge.success` (running/good/succeeded)
- `.status-badge.stopped`
- `.status-badge.pending` (starting/pending)
- `.status-badge.running` (currently mapped to accent blue — align with `#7dc4e4`)
- `.status-badge.failed`
- Job status dot colors (`jobStatusMeta.*.dot`) which mirror the same states.
- Node ready-dot colors (Ready / NotReady / Unknown) should stay consistent with the
  same palette where they map to good/warn/bad.

## Implementation note
- Define dedicated CSS custom properties (e.g. `--status-good`, `--status-stopped`,
  `--status-starting`, `--status-bad`) on both `[data-color-mode="light"]` and
  `[data-color-mode="dark"]` using the hex values above, and point the badge classes
  and job dots at those variables so a future theme change is a one-line edit.
- The badge background uses `color-mix(in srgb, <color> ~20%, transparent)` and the
  border uses the same color, so the new vars keep the existing translucent-pill look.

---
- **Status:** DONE

## Implementation (web/index.html)
- Added four status variables to both `[data-color-mode]` blocks:
  `--status-good:#8bd5ca`, `--status-stopped:#cad3f5`, `--status-starting:#7dc4e4`,
  `--status-bad:#ed8796`.
- Pointed the `.status-badge` variants at them:
  `success`->good, `stopped`->stopped, `pending`->starting, `running`->starting,
  `failed`->bad. Backgrounds keep the translucent `color-mix(... ~20%, transparent)`
  pill look; text+border use the solid var.
- Job status dots (`jobStatusMeta`) now reference the same vars.
- Node ready-dot: Ready->good, NotReady->starting, Unknown->bad.
- Verified in the browser (dark + light): running badge = `#8bd5ca`, stopped =
  `#cad3f5`; Jobs "Succeeded" = `#8bd5ca`. Rebuilt + hot-swapped the binary; pod Ready.

## Scope correction (dark-only)
The APP-17 hexes apply **only to the dark mode theme**. Light mode reverts to the
original Catppuccin-Latte theme status colors:
  `--status-good:#85b98e`, `--status-stopped:#6e738d`, `--status-starting:#dfb077`,
  `--status-bad:#e7799f`.
Verified in browser: dark running badge = `#8bd5ca`; light running badge = `#85b98e`.

## Amendment (2026-10-25): strictly catppuccin-mocha
The dark-mode status colors are now **exact catppuccin-mocha palette tokens**
(replacing the earlier approximations `#8bd5ca`/`#cad3f5`/`#7dc4e4`/`#ed8796`):

| State | Mocha token | Hex |
|-------|-------------|-----|
| good / running | `teal` | `#94e2d5` |
| stopped | `overlay2` | `#9399b2` |
| starting / in-progress | `blue` | `#89b4fa` |
| failed / error | `red` | `#f38ba8` |

Scope: **dark mode only**. Light-mode status colors are handled in **APP-18**.
The `.status-badge` icon set was also updated (see APP-18 / styleguide):
running = `cloud`, stopped = `cloud-offline`, starting/in-progress = `webhook`.
