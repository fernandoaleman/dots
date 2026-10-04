# Secrets

Started 2026-10-04 (in progress). Secrets come from **1Password** via the
`op` CLI; nothing secret is stored in this repo, encrypted or not. SSH keys
are installed as **real files** in `~/.ssh` (not served by 1Password's SSH
agent on each use).

Old source: the chezmoi repo's age-encrypted files (`encrypted_*.age`),
decrypted with an age key kept in 1Password.

## 1Password convention

Agreed 2026-10-04. `install.sh` finds items by **tag**, never by vault or
item name, so this public repo names no vault, company or work key.

| | Rule | Example |
|---|---|---|
| Account | the one signed in to `op` | |
| Vault | personal items in `Private`; work items in the work vault | `id_ed25519` in `Private` |
| Title | exactly the file name it becomes on disk | `id_ed25519` → `~/.ssh/id_ed25519` |
| Category | SSH Key for keys; Document for whole files (AWS config, `.ovpn`); API Credential for tokens | |
| Tags | `dots` plus one kind: `dots/ssh`, `dots/aws`, `dots/token`, `dots/vpn` | `dots/ssh` |
| Notes | one line saying where dots puts it | "dots: installed to `~/.ssh/id_ed25519` (600) by install.sh" |

Adding an item later is a 1Password-only change: tag it and the next
`install.sh` run picks it up. Existing items are brought in line one at a
time as each secret is reviewed. **Mac:** identical (`op` and tags).
