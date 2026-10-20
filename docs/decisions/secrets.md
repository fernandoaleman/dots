# Secrets

Done 2026-10-04. Secrets come from **1Password** via the
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
| Account | every account the profile reads (`dots_op_accounts`): **personal** reads the personal and work accounts, **work** only the work account | |
| Vault | personal items in `Private` (personal account); work items in the work account's own vault (moved there 2026-10-11) | `id_ed25519` in `Private` |
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

### `id_ed25519` (personal, Private): replaced 2026-10-11

A **new** personal key (generated in the 1Password app, Private, tag
`dots/ssh`), so the personal side depends on a key that never leaves the
personal account and personal machines. It opens Omarchy, the Mac Studio
and (for agent access) the work MacBook's Remote Login, and nothing else:
**no SSH key is on GitHub any more** (GitHub over HTTPS, [git.md](git.md)).
Never installed on the work MacBook.

### `okb_ed25519` (the old personal key, work account): kept as a work key

The previous `id_ed25519`, renamed and moved to the work account
(2026-10-11; same fingerprint). It had been everywhere (old repo, Mac
Studio, GitHub, every machine's `authorized_keys`), and its only remaining
use is the `deploy` users' `authorized_keys` on the AWS servers. Removed
from GitHub and from the `authorized_keys` of Omarchy, the Mac Studio and
the work MacBook (verified: refused by all four, the new key accepted by
the three machines). Kept installed like the other work keys (user's
choice; `deploy` is normally reached by `sudo` after logging in with an AWS
key, so removing its line from `deploy`'s `authorized_keys` and archiving
the item would retire it, if wanted later). `generate-ssh-config` doesn't
use it. The Mac Studio's own `~/.ssh/id_ed25519` file is still this key
until it gets dots.

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

## VPN profile: `<company>.ovpn` (work vault): done

- SAML sign-in (`auth-federate`), so no client certificate or key: the
  profile holds the server endpoint, the CA and settings. Compared part by
  part (hashes): 1Password's copy (2024) pointed to an **old** endpoint;
  the latest download from the AWS self-service portal (Mac Studio,
  2026-09) has the current one; the AWS VPN client's own profile (= old
  repo) is that same file with `auth-federate` stripped on import.
- User chose the latest download: streamed into the existing Document,
  retitled `<company>.ovpn`, tag `dots/vpn` added (an older, unrelated tag
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
| `JIRA_API_TOKEN` (was field `token`), `CONFLUENCE_API_TOKEN` (added, was missing), `CONFLUENCE_DOMAIN`, `CONFLUENCE_EMAIL` | Atlassian (work) |
| `CLOUDFLARE_API_TOKEN` (was field `Claude`) | Cloudflare (work) |
| `NEW_RELIC_API_KEY` (added 2026-10-13, a user key `NRAK-…`; moved from `Private`) | New Relic (work) |
| `PAGERTREE_API_TOKEN` (was field `API Token - Claude`) | PagerTree (work), moved from `Private` |
| `TODOIST_API_TOKEN` (added; `td` reads it, so no browser login) | Todoist (Personal), `Private` |
| `RAINDROP_TOKEN` (added 2026-10-20, a test token; also written to `~/.config/raindrop/token` for the bookmarks picker, [desktop.md](desktop.md#bookmarks-raindropio-with-the-raindrop-bookmarks-picker)) | Raindrop, `Private` |

Edits used piped JSON templates (values never on a command line);
existing tags and notes kept (dots line appended). One auto-saved web-form
field without ID or label was labeled `unlabeled form field` (templates
can't round-trip it; value unchanged). Verified: every exported variable
matches the Mac Studio by hash.

**Titles:** `<Service> (<account>)`: Atlassian (work) / (Personal),
Cloudflare (work) / (work devops) / (Personal), PagerTree
(work) / (work bot, moved to the work vault). The rest of
1Password follows in a later cleanup ([TODO](../TODO.md)).
In this repo `work` stands for the company's name in the real titles
(dots never names the employer; `install.sh` finds items by tag, not title).

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
- Hosts: `mac-studio` and `omarchy` (this machine) by their **Tailscale
  addresses** (`100.125.49.73`, `100.121.13.36`; changed 2026-10-10): one
  entry each, working at home (Tailscale goes direct over the LAN) and away;
  not MagicDNS names, because Macs running the AWS VPN keep Tailscale DNS
  off ([desktop.md](desktop.md)). LAN addresses (`10.0.0.100`, `10.0.0.200`
  via a DHCP reservation) remain the manual fallback. `arch` (old home
  network) dropped.
- Verified: `ssh mac-studio` works by name; `ssh -G` shows Omarchy's
  keepalive values still in effect.

**Mac** (2026-10-11): the same stowed file (`ssh` package). The keepalives
come from **Omarchy's own** `install/config/ssh-keepalive.sh`, run with sudo
from the pinned download: it writes the same
`/etc/ssh/ssh_config.d/20-omarchy-keepalive.conf`, which macOS's
`/etc/ssh/ssh_config` includes (`Include /etc/ssh/ssh_config.d/*`); verified
with `ssh -G` (15 / 3 / 10). Colima's `sshConfig` is off, or it would add
its `Include ~/.colima/ssh_config` line to `~/.ssh/config`, i.e. into this
repo ([macos.md](macos.md#docker-colima)). The work Mac has no personal key,
so `mac-studio` and `omarchy` refuse it there (by design).

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
exists and the key is already authorized. **Decided 2026-10-10:** sshd stays reachable on the LAN and over Tailscale
(key-only, rate-limited; the internet side stays closed). **Mac:** Remote Login + `authorized_keys`.

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
| `dots/gh` API Credential (work account) | read by `gh-role-login` on the work profile and handed to `gh` (macOS Keychain); not a file |
| `id_ed25519` public key (type + key) | `omarchy-setup-security-sshd --key=…`, skipped when sshd is active, hardened and the key authorized |
| — | `make ssh` once `ssh -T git@github.com` authenticates |

Titles are checked (plain names / relative paths under `~`, no `..`) before
anything is written.

## Two accounts: work items in the work account (decided 2026-10-11)

Until 2026-10-11 the "work vault" was a vault **inside the personal
account**. The work MacBook must read only the work account (no personal
data on a company machine), so the work items were **moved** into the work
account's own vault, and dots reads 1Password per machine profile:

- **Personal profile** (Omarchy, the Mac Studio): every account signed in
  to the 1Password app, personal and work (the work account was added to
  Omarchy's app). **Work profile** (the work MacBook): only the work
  account, even if the personal one were signed in. `dots_op_accounts` in
  `lib/dots.sh` lists them (excluding `DOTS_OP_PERSONAL`, the personal
  sign-in address, on the work profile); every `op` call names its account
  (`op --account`), since `op` alone only uses its default account.
- **Moved** (in the 1Password app: the CLI's `op item move` works only
  within one account, and `op item create` can't import an existing SSH key,
  tested: the item it creates is unreadable): the six work SSH keys, the
  three AWS Documents and the environments file, the VPN profile,
  `aws-role-switch`, the Atlassian, Cloudflare and PagerTree tokens, and the
  Todoist token (used for work), and the AWS SSO login (moved later the
  same day: not used by dots, but the VPN's sign-in needs its username,
  password and code, and the work Mac couldn't see it). The MFA item was
  already there. Verified
  after the move, no values printed: key fingerprints and document hashes
  match the installed files, the token env is identical (hash), the MFA
  code reads.
- **Stays personal:** `id_ed25519` (Private). It opens personal GitHub,
  Omarchy and the Mac Studio, so it never goes on the work Mac (anyone with
  admin control there could copy it). The work Mac uses GitHub over HTTPS
  with `gh` (see [git.md](git.md)); AWS hosts use the work keys
  (`generate-ssh-config` sets a per-host `IdentityFile`), never
  `id_ed25519`.
- `install.sh`'s secrets step (`install_secrets`) runs on both systems; the
  VPN plugin and sshd parts are Omarchy's (the Mac imports the VPN profile
  by hand, checklist step 5). `aws-role-login` searches every account for
  the `dots/aws-mfa` item.
- Tested: Omarchy (both accounts) reports every file up to date; the work
  Mac installed its keys, AWS files and tokens from the work account only;
  re-runs change nothing.

## Private words: a lint check, not a habit (decided 2026-10-19)

The repo is public, and the employer must never be named in it. That rule
was followed by habit until a pre-push scan on 2026-10-19 found the
company's name in 1Password item titles, the VPN profile's name and the old
Slack web app's URL (commits 2026-10-03..17). Fixed by making the repo
private, rewording the docs (`work`, `<company>.ovpn`), rewriting history
with `git filter-repo --replace-text` (one case-insensitive rule), a
force-push of `master` and the tags (Releases follow their tags), and
making it public again. The work key's file name `okb_ed25519` was kept
(user's call: it doesn't name the company on its own).

So a prek hook enforces it: `scripts/check-private-words`, at pre-commit
(staged files) and commit-msg (the message), so also in `make lint`. The
words are in a **machine-local** list, `~/.config/dots/private-words` (mode
600; one extended regex per line, case-insensitive), because a list in the
repo would publish them.

- **Fails closed:** no list means no commit, so a machine without one can't
  commit unchecked. A list with only comments is an explicit opt-out (it
  warns). A broken pattern fails too (grep's error exit isn't read as
  clean).
- **Tested** with controls on both systems: a hit in a file and in a
  message fails, a clean tree passes, `\b` word boundaries work in macOS's
  BSD grep and under `/bin/bash` 3.2.
- **Not covered:** commits made with `--no-verify` or on a machine without
  the hooks (`make setup`). Before a push, also scan
  `git log -p origin/master..` by hand.
- **Later:** keep the list in 1Password and have `install.sh` write it
  ([TODO](../TODO.md)).
