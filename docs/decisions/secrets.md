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
| Title | exactly the file name it becomes on disk; the path under `~` when the bare name isn't unique | `id_ed25519` → `~/.ssh/id_ed25519`; `.aws/config` → `~/.aws/config` |
| Category | SSH Key for keys; Document for whole files (AWS config, `.ovpn`); API Credential for tokens | |
| Tags | one nested tag per item: `dots/ssh`, `dots/aws`, `dots/token`, `dots/vpn` (the parent `dots` is implied: `op item list --tags dots` also matches `dots/ssh`, verified) | `dots/ssh` |
| Notes | one line saying where dots puts it | "dots: installed to `~/.ssh/id_ed25519` (600) by install.sh" |

Adding an item later is a 1Password-only change: tag it and the next
`install.sh` run picks it up. Existing items are brought in line one at a
time as each secret is reviewed. **Mac:** identical (`op` and tags).

**1Password CLI limit:** `op item edit` can't edit SSH Key items (*"SSH
Key item editing in the CLI is not yet supported"*), so their tags and
notes are set in the 1Password app.

## Comparing copies

The old repo, 1Password and the Mac Studio (reachable over SSH) can hold
different copies. Comparisons run where the secret lives and return only
non-secret facts (SHA-256 hashes, key fingerprints, section/key names with
values elided); when copies differ, the user decides which to keep.

## SSH keys

None of the keys has a passphrase.

### `id_ed25519` (personal, Private): done

- 1Password item matches the old repo (fingerprint); the duplicate
  "My SSH Key" (same fingerprint) was archived.
- Tag `dots/ssh` and the note set in the 1Password app (verified with `op item get`).
- Installed here by hand for now (`op read "…/private key?ssh-format=openssh"`
  to `~/.ssh/id_ed25519` 600, public key to `.pub` 644); `install.sh`
  takes over once the SSH setup (`IdentityFile`, agent, sshd) is decided.
  It already logs in to the Mac Studio without a password.

### Work keys (integration, QA, staging, production): done

- Same fingerprint in the old repo, the Mac Studio and 1Password.
- Moved from `Private` to the work vault (`op item move`; fingerprints
  verified unchanged), tag `dots/ssh` and the note set in the app
  (verified).

### lab and pricing-agent-sandbox keys: added

- Missing from 1Password; the old repo and the Mac Studio held the same
  keys (fingerprints). Both are work keys (pricing-agent-sandbox: the AWS
  sandbox key of a work project), so they went into the work vault.
- Copied from the Mac Studio to `~/.ssh` (600) with `scp`, imported in the
  1Password app as SSH Key items (tag `dots/ssh`, note), verified
  (fingerprint, tag, note), then the local copies were deleted; dots
  installs them again later.

### Obsolete keys: archived

Not in the old repo, not on the Mac Studio, not referenced anywhere; the
user confirmed them obsolete. Archived in 1Password (recoverable): an
ed25519 SSH Key and a different RSA key (a Document) that shared one
`.pem` name, and a DevOps ed25519 key.

**SSH keys in 1Password: complete.** Every key dots installs is an SSH Key
item tagged `dots/ssh` (one personal in `Private`, six work in the work
vault); `install.sh` support comes with the SSH setup decisions.

## AWS

The daily MFA flow: `~/.aws/credentials` `[aws-role-switch]` holds the
long-lived access key; `aws-role-login` writes temporary MFA credentials
into `[mfa]`; the role profiles in `~/.aws/config` use
`source_profile = mfa`. `[mfa]` is regenerated daily and is not stored.

### `.aws/config` (role profiles): done

- Old repo and Mac Studio identical; 1Password's "aws config" (Private)
  was an older version missing four profiles. User chose the current one.
- Contents streamed from the Mac Studio into the existing Document
  (`op document edit … -`, never displayed), retitled `.aws/config`, tag
  `dots/aws`, note, moved to the work vault; hash verified equal to the
  Mac Studio's. No Rackspace profile in it.
