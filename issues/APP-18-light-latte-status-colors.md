# APP-18: Light-mode (catppuccin-latte) status colors + chrome

- **Type:** UX / Aesthetic
- **Summary:** Define the **light mode** theme using **strictly catppuccin-latte**
  palette tokens, mirroring the dark-mode work in APP-17. Also applies the shared
  chrome changes (header, tabs, active tab, action buttons) with latte values.
- **Severity:** Low (aesthetic, non-blocker)
- **Effort:** Trivial
- **Depends on:** APP-17 (dark/mocha) for parity of roles.

## Light-mode (catppuccin-latte) status colors
| State | Latte token | Hex |
|-------|-------------|-----|
| good / running | `teal` | `#179299` |
| stopped | `overlay2` | `#6c6f85` |
| starting / in-progress | `blue` | `#1e66f5` |
| failed / error | `red` | `#d20f39` |

## Chrome (both modes, per-token)
| Element | Dark (mocha) | Light (latte) |
|---------|--------------|---------------|
| App header background | `crust` `#11111b` | `crust` `#dce0e8` |
| Tab list background | `mantle` `#181825` | `mantle` `#cdd1da` |
| Active tab background | `base` `#1e1e2e` | `base` `#eff1f5` |
| Active tab accent (fg) | `blue` `#89b4fa` | `blue` `#1e66f5` |

## Action button colors (per mode)
| Action | Dark (mocha) | Light (latte) | Octicon |
|--------|--------------|---------------|---------|
| View logs | `rosewater` `#f5e0dc` | `rosewater` `#b4637a` | `log` |
| Terminate pods | `peach` `#fab387` | `peach` `#fe640b` | `issue-reopened` |
| Delete deployment | `red` `#f38ba8` | `red` `#d20f39` | `crosshair` |

## Shared chrome / icons (both modes)
- App header octicon: `rocket`.
- Deployments tab: `cloud` icon to the left of the "Deployments" text.
- Status badge icons: running = `cloud`, stopped = `cloud-offline`,
  starting/in-progress = `webhook`.
- Jobs tab: same status-icon mapping; "View job output" = `log` icon + rosewater color.

All of the above is documented in `docs/styleguide.md`.

---
- **Status:** DONE
