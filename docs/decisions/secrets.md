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
| Category | by what the secret is: **SSH Key** for keys; **Document** for a whole file whose contents are the secret and don't change (AWS config, `.ovpn`); **API Credential** for values dots writes into a file that other tools also change (`~/.aws/credentials`, tokens) | |
| Tags | one nested `dots/…` tag per item (other tags an item already has are kept): `dots/ssh`, `dots/aws`, `dots/token`, `dots/vpn` (the parent `dots` is implied: `op item list --tags dots` also matches `dots/ssh`, verified) | `dots/ssh` |
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

### `.aws/config.sso` (SSO profiles): done, review ~2026-11-03

- Not read by the AWS CLI unless `AWS_CONFIG_FILE` points at it (nothing
  in the old setup did); identical in the old repo and on the Mac Studio;
  missing from 1Password. SSO is rarely used but kept (user's choice);
  usage review in the [TODO](../TODO.md).
- While streaming it into a new Document (work vault, `.aws/config.sso`,
  tag `dots/aws`, note): removed `[rackspace]` (no longer used) and merged
  the 10 identical `sso-session` blocks (same start URL, region, scopes)
  into one, so one `aws sso login` covers all 10 profiles. Hash verified;
  the AWS CLI parses all 10 profiles, each on the single session.

### `aws-role-switch` (long-lived key in `~/.aws/credentials`): done

- `~/.aws/credentials` is not stored as a Document: the daily login
  rewrites its temporary `[mfa]` section, so a stored copy would be stale
  the next day (and would hold a session token). Only the lasting secret
  is stored, once: API Credential `aws-role-switch` (work vault, tag
  `dots/aws`, fields `access key id` / `secret access key`, note).
  Rotating the key is a single update.
- Old repo and Mac Studio held the same key (hash); the old 1Password item
  "AWS Access Key (faleman)" was a different, invalid key from the retired
  Rackspace account: archived. New item created from a JSON template piped
  from the Mac Studio (the key never on a command line); verified with
  `aws sts get-caller-identity`.
- `install.sh` will write `~/.aws/credentials` (600) with only
  `[aws-role-switch]`. **Condition:** the old `aws-role-login` overwrote
  lines 2-4 with `sed`, assuming `[mfa]` on line 1; on such a file it would
  overwrite the long-lived key. The port must write `[mfa]` with
  `aws configure set … --profile mfa` (creates the section if missing,
  touches nothing else).

### MFA code: "AWS Role Switch" login, tag `dots/aws-mfa`

The Login item holding the MFA one-time password (also used for the
console sign-in) stays where it is, tagged `dots/aws-mfa` with a note;
`aws-role-login` finds it by that tag. Verified: key from `aws-role-switch`
+ `mfa_serial` from `.aws/config` + the item's OTP gives a 12-hour session
with no typing ([bin.md](bin.md)).

DRY: its IAM section duplicated the `aws-role-switch` key (same hashes),
so the section was deleted; the item keeps only the console sign-in and the
MFA code. The key lives only in `aws-role-switch`.

## VPN profile: `1000bulbs.ovpn` (work vault): done

- SAML sign-in (`auth-federate`), so no client certificate or key: the
  profile holds the server endpoint, the CA and settings. Compared part by
  part (hashes): 1Password's copy (2024) pointed to an **old** endpoint;
  the latest download from the AWS self-service portal (Mac Studio,
  2026-09) has the current one; the AWS VPN client's own profile (= old
  repo) is that same file with `auth-federate` stripped on import.
- User chose the latest download: streamed into the existing Document,
  retitled `1000bulbs.ovpn`, tag `dots/vpn` added (an older, unrelated tag
  kept), note; hash verified. Install path is decided with the AWS VPN
  plugin ([TODO](../TODO.md)).
- A separate GitHub-runner VPN profile (own client key) on the Mac Studio
  was left out (not used).

## API tokens (`dots/token`): done

Old zsh `40-api-tokens` variables (rclone ones dropped with rclone). Old
repo and Mac Studio identical; all four tokens valid (HTTP 200 from each
service's "who am I" endpoint). Jira and Confluence use two different
tokens (the old comment saying "same token" was outdated).

**Pattern (DRY):** each token stays in the Login item it belongs to; the
field's **label is the variable name**; the item is tagged `dots/token`.
`install.sh` exports every field of those items whose label matches
`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$` (ALL_CAPS with an underscore, so a field
like `MCP` is not exported) into one generated env file (600, outside the
repo) that bash loads. A new token is a 1Password-only change.

| Variable | Item (work vault) |
|---|---|
| `JIRA_API_TOKEN` (was field `token`), `CONFLUENCE_API_TOKEN` (added, was missing), `CONFLUENCE_DOMAIN`, `CONFLUENCE_EMAIL` | Atlassian (1000Bulbs) |
| `CLOUDFLARE_API_TOKEN` (was field `Claude`) | Cloudflare (1000Bulbs) |
| `PAGERTREE_API_TOKEN` (was field `API Token - Claude`) | PagerTree (1000Bulbs), moved from `Private` |

Edits used piped JSON templates (values never on a command line);
existing tags and notes kept (dots line appended). One auto-saved web-form
field without ID or label was labeled `unlabeled form field` (templates
can't round-trip it; value unchanged). Verified: every exported variable
matches the Mac Studio by hash.

**Titles:** `<Service> (<account>)`: Atlassian (1000Bulbs) / (Personal),
Cloudflare (1000Bulbs) / (1000Bulbs devops) / (Personal), PagerTree
(1000Bulbs) / (1000Bulbs bot, moved to the work vault). The rest of
1Password follows in a later cleanup ([TODO](../TODO.md)).

## SSH setup

### `~/.ssh/config`: `ssh` stow package (decided 2026-10-04)

Omarchy ships no `~/.ssh/config` (so no include line; not in the config
pattern tally), only system defaults in
`/etc/ssh/ssh_config.d/20-omarchy-keepalive.conf` (`ServerAliveInterval
15`, `ServerAliveCountMax 3`, `ConnectTimeout 10`, for its `ssh` reconnect
wrapper), which still apply. Ours (`ssh/.ssh/config`, public, generic):

- `Include ~/.ssh/aws` (written by `generate-ssh-config`) and
  `Include ~/.ssh/config.local` (machine-local, not committed).
- **`IdentityFile` stays per host in the generated `~/.ssh/aws`**: it is
  written from one environment-to-key table, so it is already a single
  source; host patterns in the main config would put work names in this
  public file and break on irregular names.
- **No SSH agent:** no key has a passphrase, so there is nothing to
  remember; `IdentityFile` (or the default `id_ed25519`) picks the key.
  Dropped `AddKeysToAgent` (and `UseKeychain` on the Mac). If keys ever get
  passphrases: enable `gcr-ssh-agent.socket` + `AddKeysToAgent yes`
  ([troubleshooting](../guides/troubleshooting.md)).
- `Host *`: **kept** `SetEnv TERM=xterm-256color` (inside tmux `TERM` is
  `tmux-256color`, which many servers lack; `TERM` is sent regardless of
  the server's `AcceptEnv`) and `StrictHostKeyChecking accept-new` (new
  hosts added, changed keys refused). **Dropped** `ServerAliveInterval 120`
  and `TCPKeepAlive no` (would override Omarchy's faster keepalives).
- Hosts: `mac-studio` (`10.0.0.100`) and `omarchy` (this machine,
  `10.0.0.200` via a DHCP reservation on the router); both `HostName`s
  become their Tailscale names once Tailscale is set up. `arch` (old home
  network) dropped.
- Verified: `ssh mac-studio` works by name; `ssh -G` shows Omarchy's
  keepalive values still in effect.

**Mac:** same file; no Omarchy keepalive file there, so the Mac setup adds
the same 15 / 3 / 10 values.

### Incoming SSH (sshd): Omarchy's script in `install.sh` (decided 2026-10-04)

State here: `omarchy-setup-security-sshd` was run at install (2026-10-02):
sshd enabled, `ufw limit 22/tcp` (IPv4 + IPv6, `omarchy-sshd`), the key
authorized, then password logins disabled
(`/etc/ssh/sshd_config.d/10-omarchy-hardening.conf`). The authorized key,
GitHub's `fernandoaleman.keys` and 1Password's `id_ed25519` are the same
key (fingerprint). Verified: `ssh omarchy` (10.0.0.200) logs in by key.

`install.sh` (secrets step) runs
`omarchy-setup-security-sshd --key="$(op read "op://Private/id_ed25519/public key")"`
(only that one key), skipped when sshd is active, the hardening file
exists and the key is already authorized. **Later, with Tailscale:**
decide whether sshd answers on LAN + Tailscale or Tailscale only (outgoing
SSH is unaffected either way; LAN devices without Tailscale would be
refused in the second case). **Mac:** Remote Login + `authorized_keys`.

### `generate-ssh-config` environment table: done

Document `.config/generate-ssh-config/environments` (work vault, tag
`dots/aws`, note): the five environments with their AWS profile and SSH key,
taken from the Mac Studio's script (same as the old repo). See
[bin.md](bin.md).

## `install.sh` secrets step (decided 2026-10-04)

Runs only when `op` can read the vaults (otherwise a warning with the
sign-in steps, [setup](../setup/omarchy.md)); everything is found by tag
and no value is printed. Files go through a temp file next to the target
(not `/tmp`); a file whose contents differ is backed up as
`<file>.bak.<stamp>` first (1Password is the source of truth); unchanged
files are left alone.

| Tag / item | Installed to |
|---|---|
| `dots/ssh` SSH Keys | `~/.ssh/<title>` (600), `<title>.pub` (644) |
| `dots/*` Documents whose title is a path | `~/<title>` (600): `.aws/config`, `.aws/config.sso`, `.config/generate-ssh-config/environments` (the VPN profile has no path yet, so it is skipped) |
| `dots/aws` API Credentials | `~/.aws/credentials` profile `[<title>]`, replaced in place (other profiles such as `[mfa]` kept; values passed to `awk` through its environment, not the command line) |
| `dots/token` fields named like variables | `~/.config/dots/env` (600), loaded by `~/.bashrc.dots` |
| `id_ed25519` public key (type + key) | `omarchy-setup-security-sshd --key=…`, skipped when sshd is active, hardened and the key authorized |
| — | `make ssh` once `ssh -T git@github.com` authenticates |

Titles are checked (plain names / relative paths under `~`, no `..`) before
anything is written.
