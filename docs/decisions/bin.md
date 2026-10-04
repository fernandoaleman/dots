# Bin scripts

Done 2026-10-04. Old source: `dotfiles/dot_local/bin/`
(installed to `~/.local/bin`) plus `tmux-ssh` (downloaded from upstream by
`.chezmoiexternal.toml`). Omarchy puts `~/.local/bin` on `PATH`
(`default/bash/envs`).

Already decided elsewhere: `cache-sesh-dirs` (dropped with sesh,
[tmux.md](tmux.md)); `open-herdr.sh` and the `hdl`/`hdlm`/`hds`/`hsl`
links (Mac phase, [packages.md](packages.md)).

## colortest: dropped

Printed the 7 basic terminal colors as regular, bold and underlined text
with the `tput` commands. **Dropped** (2026-10-04): a debugging toy; Omarchy
has no equivalent (its color commands are theme/bar tools), and colors come
from Omarchy's themes. One-liner in [troubleshooting](../guides/troubleshooting.md).

## create-ansible-docker-image: dropped

Scaffolded a repo for an Ansible test Docker image (`image:tag` argument):
Dockerfile (Ubuntu + Ansible via pip + systemd), a GitHub Actions build and
Docker Hub push, lint configs, README, initial commit. **Dropped**
(2026-10-04): work tooling, not a dotfile, and it hard-codes the
employer's Docker Hub and GitHub orgs (not for this public repo). Its home
is a work repo; until then it stays in the old dotfiles repo.

## td-login: dropped

A wrapper: print `td auth status` when logged in, else run
`td auth login "$@"` (browser OAuth; e.g. `--read-only`) and re-check.
Nothing from 1Password. **Dropped** (2026-10-04): `td auth login` /
`td auth status` do the same; the [setup step](../setup/omarchy.md) and
`install.sh`'s logged-out warning cover it. **Mac:** same `td` commands.

## aws-sso-login: dropped (two commands in troubleshooting)

Ran `aws sso login --profile <name>` for 10 work profiles. **Dropped**
(2026-10-04): against today's `~/.aws/config` (role profiles, no SSO
settings) every login fails, so it only worked in an older setup; SSO is
rarely used, since role + MFA (`aws-role-login`) covers the CLI. The SSO
config stays (`~/.aws/config.sso`, one merged session, from 1Password;
ignored by the AWS CLI unless `AWS_CONFIG_FILE` points at it); the two
commands are in [troubleshooting](../guides/troubleshooting.md). A
role/SSO mode switch (renaming config files) was rejected: rare use, and
forgetting to switch back would break the daily role login. Usage review
~2026-11-03 ([TODO](../TODO.md)).

## aws-role-login: `bin` package, code from 1Password (daily use)

**Used every day**, with the Chrome extension *AWS Extend Switch Roles*
(console side, unaffected). Ported 2026-10-04 to
`bin/.local/bin/aws-role-login`, the first script of the `bin` stow package
(linked into `~/.local/bin`):

| | Old | New |
|---|---|---|
| Usage | `aws-role-login 123456` | `aws-role-login` (code from 1Password: the one item tagged `dots/aws-mfa`, `op item get --otp`); `aws-role-login 123456` still works |
| MFA device | ARN hard-coded (account ID) | `mfa_serial` read from `~/.aws/config` (profile `aws-role-switch`) |
| Writing `[mfa]` | `sed` over lines 2-4 of `~/.aws/credentials` (fragile; would overwrite the long-lived key on a file without `[mfa]` first) | `aws configure set … --profile mfa` (creates or updates only that section) |
| Session | 12 h | 12 h (`DURATION`; IAM users can go to 36 h) |

Same security model (the code still comes from the MFA device in
1Password; 1Password may ask to approve the terminal), one step less.
Tested against temporary AWS files: creates `[mfa]` when missing, updates
it in place, a role profile assumes its role, the long-lived key is
untouched, malformed codes are rejected. Nothing work-specific in the
script. **Mac:** same script (`date -d` falls back to the raw expiry time).

## generate-ssh-config: `bin` package, runs after `aws-role-login` (daily use)

For each work environment it queries running EC2 instances (`aws ec2
describe-instances` with that environment's profile), writes `~/.ssh/aws`
(one `Host` per instance: Name tag, `-NN` suffix for duplicates, private
IP, the environment's user and key as `IdentityFile`, so `ssh <host>` needs
no `-i`) and `~/.config/tmux-ssh/tmux-ssh.conf` (groups per environment
from the Name tags: all ASG servers, `-web`, `-cron`, `-workers`,
`-console`). Safe by design: a failed query changes nothing, an empty
result is never written, both files are replaced together with backups.

Ported 2026-10-04 to `bin/.local/bin/generate-ssh-config` (the Mac
Studio's installed version, identical logic to the old repo):

- **Generic:** the environment table (employer names) moved to
  `~/.config/generate-ssh-config/environments`, one line per environment
  (`<environment> <aws-profile> <ssh-key> [user, default ubuntu]`), a
  Document in 1Password (work vault, tag `dots/aws`) installed by
  `install.sh`. A new environment is one line there.
- **Runs automatically after `aws-role-login`** (user's choice), the one
  moment the MFA session is guaranteed fresh; `aws-role-login
  --no-ssh-config` skips it. (A daily timer was rejected: it would fail on
  days the login hadn't happened yet.)
- Verified end to end in a throwaway home: 49 hosts and 18 groups,
  identical to the Mac Studio's files (IPs excluded).

## tmux-ssh: `install.sh`, pinned commit

The user's own project ([tmux-ssh/tmux-ssh](https://github.com/tmux-ssh/tmux-ssh),
MIT): `tmux-ssh host1 host2 …` or a group from
`~/.config/tmux-ssh/tmux-ssh.conf` (written by `generate-ssh-config`)
opens one synchronized SSH pane per host (`prefix =` toggles
synchronization). The old setup downloaded `master` weekly
(`.chezmoiexternal.toml`). Omarchy has nothing like it.

**Kept** (2026-10-04): `install.sh` downloads it at a **pinned commit**
(`TMUX_SSH_COMMIT`, with its `TMUX_SSH_SHA256`) to `~/.local/bin`, like
the tmux plugins: re-runs skip it when the hash matches, a download that
doesn't match is refused. Bump both deliberately; pin a release tag once
the project has them ([TODO](../TODO.md)). It runs plain `ssh <host>`, so
the SSH key setup from the secrets section applies to every pane.

**Mac:** same download (`sha256sum` from Homebrew coreutils, or
`shasum -a 256`).

## backup-to-thumb-drive / import-from-thumb-drive: dropped

**Dropped** (2026-10-04): **remote desktop** from the Mac into this machine
(over Tailscale, [TODO](../TODO.md)) replaces carrying work between
computers. Recorded in full so they can be brought back.

What they did (`rsync`, mirror images; backup = machine to drive, import =
drive to machine, drive at `/Volumes/Backup/laptop/`):

| What | rsync flags | Note |
|---|---|---|
| `~/Downloads/` | `-aWv --info=progress2` | copy |
| `~/code/` | `-aWv --delete --info=progress2` | exact mirror (extra files removed) |
| `~/.claude/` | `-aWv --update --info=progress2` + excludes | only newer files |
| `~/.local/state/zsh/history` | `-aWv --info=progress2` | shell history |

`~/.claude` excludes (runtime state and caches; transcripts, file-history,
skills, plugins, memory, todos, CLAUDE.md and settings, including
`settings.local.json`, do travel):

```sh
CLAUDE_EXCLUDES=(
  --exclude 'cache/' --exclude 'paste-cache/' --exclude 'usage-data/'
  --exclude 'telemetry/' --exclude 'shell-snapshots/' --exclude 'session-env/'
  --exclude 'daemon/' --exclude 'daemon.log' --exclude 'debug/' --exclude 'ide/'
)
```

**To bring back on Omarchy:** a `bin` stow package (to `~/.local/bin`);
removable drives mount under `/run/media/$USER/<label>` (not `/Volumes`);
`~/Work` instead of `~/code`; `~/.bash_history` instead of the zsh
history. Originals: `dotfiles/dot_local/bin/executable_{backup-to,import-from}-thumb-drive.tmpl`
and `dotfiles/.chezmoitemplates/claude_sync_excludes.bash` in the old repo.

## push-to-mac-studio / pull-from-mac-studio: dropped (revisit in the Mac phase)

**Dropped** (2026-10-04) on Omarchy: remote desktop replaces syncing work
between machines. **Mac phase:** revisit; if remote desktop from the Mac
into this machine works, they may not be needed, but the Macs may still
want them. Recorded in full.

Mirror images over SSH to `faleman@mac-studio` (`10.0.0.100`, home LAN):
push = this machine to the Mac Studio, pull = the reverse. Steps:

1. Refuse to run on the Mac Studio (local IPs include `10.0.0.100`;
   `ifconfig` on macOS, `ip -4 -o addr show` on Linux).
2. Check SSH without prompting:
   `ssh -q -o BatchMode=yes -o ConnectTimeout=5 faleman@mac-studio exit`.
3. Confirm: *"Are you sure? (yes/no)"* (the other side is overwritten).
4. `rsync -avz --progress`:
   - `~/code/` with `--delete` (exact mirror);
   - `~/.claude/` with `--update` and the `CLAUDE_EXCLUDES` list (see the
     thumb-drive scripts above);
   - Macs only: `~/.herdr/` with `--delete` (herdr's git worktrees; their
     registrations live in `<repo>/.git/worktrees/`, carried by the
     `~/code` sync, and reference absolute `/Users/faleman` paths, hence
     Mac-to-Mac only) and the zsh history.

**To bring back:** a `bin` stow package; `~/Work` instead of `~/code` on
Omarchy; bash history instead of zsh's. Originals:
`dotfiles/dot_local/bin/executable_{push-to,pull-from}-mac-studio.tmpl` in
the old repo.
