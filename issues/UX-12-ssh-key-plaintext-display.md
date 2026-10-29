# UX-12: SSH key modal shows the full key in plaintext

- **Type:** UX
- **Summary:** The SSH-key modal loads and displays the stored public key verbatim in a textarea. Public keys are less sensitive, but the modal gives no copy button and treats it like a secret, inconsistent with the tokens UX.
- **Severity:** Low
- **Effort:** Low

## Why this issue should be addressed / ranked
Minor consistency/ergonomics: a public key should be easy to copy (it is meant to be shared) and the display should make clear it is a *public* key. Low sensitivity, low impact.

## Justification for the severity level
Low. Public key material; no real exposure. The issue is ergonomics and labeling.

## Dependencies
- **Depends on:** none.
- **Enables:** nothing.

## Justification for the effort level
Low. Add a "Copy" button and a "public key" label/hint; optionally mask the body until a "reveal" toggle. A few lines.

## Review — 2026-10-27 (UX re-check)

**Status: 🔴 STILL PRESENT** (re-verified 2026-10-27)

Evidence in `web/index.html`:

`openSSHKeyModal` sets `sshKeyInput.value = data['ssh-public-key']` (plaintext). Title already says 'SSH Public Key' (labeling partly addressed) but there is no copy button for the stored key and no reveal/mask.
