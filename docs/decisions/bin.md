# Bin scripts

Started 2026-10-04 (in progress). Old source: `dotfiles/dot_local/bin/`
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

## aws-sso-login: deferred to the secrets section

Runs `aws sso login --profile <name>` for each of 10 hard-coded work
profiles. Important (not daily; `aws-role-login` is); open to a better
way. Deferred (2026-10-04)
to the secrets section, with `~/.aws/config` (encrypted in the old repo):

- With AWS CLI v2, profiles that share one `[sso-session]` share one
  cached login: a single `aws sso login` (or `--sso-session <name>`) covers
  them all (`aws sso login help`: *"By default, this command will login
  using the SSO session configured as part of the requested profile"*).
  If the config uses (or can use) an `sso-session`: drop the script.
- Otherwise: a small version looping over `aws configure list-profiles`.
- Either way, no profile names in this public repo (they name the
  employer).

`aws` (v2) comes from our mise tools; Omarchy has nothing for AWS.
**Mac:** same.

## aws-role-login: deferred to the secrets section (daily use)

**Used every day**, together with the Chrome extension *AWS Extend Switch
Roles* (console role switching). Run as `aws-role-login <MFA code>`: calls
`aws sts get-session-token` with a long-lived key profile, the MFA device
(kept in 1Password) and the code, then `sed`s the temporary key, secret and
token over **lines 2-4** of `~/.aws/credentials` (its own comment: the MFA
profile *"must be at the TOP"*), which the role profiles then use.
Deferred (2026-10-04) to the secrets section, first among the AWS items:

- Not in this public repo as-is: it hard-codes the AWS account ID (MFA
  ARN); editing fixed lines of the credentials file is fragile.
- Better ways to evaluate: native AWS CLI (`mfa_serial` + `role_arn` /
  `source_profile` in `~/.aws/config`: the CLI prompts for the code once
  and caches in `~/.aws/cli/cache` until expiry), or fully automatic with
  1Password (`op item get <item> --otp`, e.g. via `credential_process`).
- Check how the chosen way fits the AWS Extend Switch Roles extension
  (console side; its config lives in the extension / Chrome sync).

**Mac:** same AWS CLI and `op`.

## generate-ssh-config: kept, deferred to the secrets section (daily use)

**Run every day.** For each work environment it queries running EC2
instances (`aws ec2 describe-instances` with that environment's profile),
writes `~/.ssh/aws` (one `Host` per instance: Name tag, `-NN` suffix for
duplicates, private IP, user `ubuntu`, the environment's key as
`IdentityFile`, so `ssh <host>` needs no `-i`) and
`~/.config/tmux-ssh/tmux-ssh.conf` (groups per environment from the Name
tags: all ASG servers, `-web`, `-cron`, `-workers`, `-console`; used with
tmux-ssh and `prefix =` synchronize panes). Safe by design: a failed query
changes nothing, an empty result is never written, both files are replaced
together with backups.

**Kept** (2026-10-04), implemented in the secrets section with its
dependencies: AWS credentials (`aws-role-login`), the SSH keys as real
files from 1Password, `~/.ssh/config` including `~/.ssh/aws`, and
`tmux-ssh`. Plan:

- The script goes in dots **generic**: the environment, profile and key
  table (employer names) moves to a machine-local file (gitignored, or
  from 1Password).
- **Run it automatically, daily** (user's idea): on Omarchy a systemd user
  timer (cron isn't Omarchy's way); it needs valid AWS credentials, so
  the timer can only succeed after the day's `aws-role-login` (the script
  already aborts safely on expired credentials). Alternative: run it right
  after `aws-role-login` succeeds. Decide in the secrets section.
- **Where `IdentityFile` lives** (user's idea): instead of one per host
  in the generated `~/.ssh/aws`, maybe set it once in the main
  `~/.ssh/config` (e.g. per environment host pattern). Discuss when the
  SSH keys are set up.

**Mac:** same script (bash 4+ from Homebrew); a launchd agent instead of
the systemd timer.

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
