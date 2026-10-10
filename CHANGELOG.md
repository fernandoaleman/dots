# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
`1.0.0` marks Omarchy complete, including secrets via 1Password, verified
by a clean install; macOS support comes in later `1.x` releases.

## [Unreleased]

### Added

- Remote desktop host: Sunshine (Moonlight streaming over the LAN and
  Tailscale), installed as Omarchy's installer does it, with its open bugs
  fixed (real unit name, no double start, no browser-wide certificate
  bypass). It streams the middle desk monitor (`output_name = DP-2`).

## [1.0.0] - 2026-10-04

Omarchy complete: a fresh Omarchy install plus `install.sh` (and signing in
to 1Password) gives the whole machine, secrets included. Verified by a
clean install in a VM.

### Added

#### Secrets from 1Password

- `install.sh` secrets step, everything found by 1Password tag (no vault,
  company or key names in this repo): SSH keys (`dots/ssh`), `~/.aws`
  files and credentials (`dots/aws`), an API token env file
  (`~/.config/dots/env`, loaded by bash; `dots/token`) and the VPN
  profile (`dots/vpn`); incoming SSH via Omarchy's sshd script; the repo
  remote switched to SSH. Skipped with instructions when 1Password isn't
  signed in.
- Todoist logs in through `TODOIST_API_TOKEN`: no `td auth login` step.

#### AWS and SSH

- `bin` package (`~/.local/bin`): `aws-role-login` starts the daily AWS
  MFA session with the code from 1Password (no typing), writes `[mfa]`
  with `aws configure set`, then runs `generate-ssh-config`, which writes
  `~/.ssh/aws` (per-host `IdentityFile`) and tmux-ssh groups from running
  EC2 instances (environment table from 1Password).
- `ssh` package: `~/.ssh/config` (includes the generated `~/.ssh/aws` and
  `~/.ssh/config.local`; hosts `mac-studio`, `omarchy`; `TERM` fix and
  `accept-new`; Omarchy's keepalives kept; no agent).
- `tmux-ssh` in `~/.local/bin` at a pinned commit (SHA-256 checked).

#### Apps and system

- AWS VPN: the `omarchy-aws-vpn-client` plugin from its repo (daemon via
  `omarchy-pkg-aur-add`, the plugin's `setup --no-keybind`), its profile
  from 1Password, and Super+Shift+V for its panel.
- Tailscale via Omarchy's installer.
- Sudoless Docker via Omarchy's opt-in script (reboot deferred).
- `mariadb-clients` (`mysql`, `mysqldump`).
- `ruby` package: `~/.gemrc` with `gem: --no-document`.

#### Shell

- `mysql`/`mysqldump` aliased to MariaDB's current names (no deprecation
  warning); an `ncdu` function pointing to Omarchy's `dua i`.

### Fixed

Found by the clean-install test:

- On a brand-new Omarchy, `install.sh` runs Omarchy's update first when
  pacman has no package lists yet.
- `install.sh` stows with `--no-folding` and creates `~/.ssh` first: Stow
  had linked the whole `~/.ssh` into the repo, so the installed keys landed
  in the git working tree (nothing committed).
- The `bin` scripts were never committed (`.gitignore`'s `*.local` matched
  the `bin/.local/` folder); folders named `.local` are re-included and
  `make doctor` reports any package file that isn't committed.
- The secrets step no longer triggers 1Password's interactive "add an
  account" prompt on a machine without a 1Password account.

## [0.2.0] - 2026-10-04

Desktop: monitors and app workspaces, Docker networks clear of the work
VPN, Todoist agent skills, and hidden files in Neovim.

### Added

- `hypr` package: `monitors.dots.lua` (loaded by one line in Omarchy's
  `monitors.lua`): the three desk monitors matched by description, left to
  right at scale 1.6, with workspaces 1–3 / 4–6 / 7–9 bound to left /
  middle / right; other screens keep Omarchy's defaults.
- `hypr` package: `hyprland.dots.lua` pins apps to workspaces (Chrome 1,
  tmux terminal 4, Slack and Teams 7, Outlook 8, Spotify 9); Super+Alt+Return gains
  `--app-id=tmux` so its terminal can be matched.
- `install.sh` installs Spotify with Omarchy's installer.
- `install.sh` fills Omarchy's empty XCompose name/email snippets
  (CapsLock, Space, n / e) from the git identity.
- `docs/TODO.md`: ideas and follow-ups for after the migration.
- `install.sh` installs the Todoist CLI's agent skills for Claude Code
  (`~/.claude/skills`) and Codex/opencode (`~/.agents/skills`).
- `install.sh` moves Docker's bridge to `172.31.0.1/16` and other Docker
  networks to `192.168.128.0/17` (a work VPN routes `172.17.x.x`), with
  container DNS and firewall following; `make doctor` reports if reset.

### Changed

- Neovim shows and searches hidden files by default (explorer, file and
  grep pickers, grug-far); git-ignored files stay hidden and `.git/` is
  never shown (new `neo-tree.dots.lua` and `snacks.dots.lua`). In
  neo-tree, `H` toggles hidden and `I` toggles git-ignored files.

### Removed

- espanso (stock config only); Omarchy's Compose key covers name/email.

## [0.1.0] - 2026-10-03

First release: a fresh Omarchy install plus `install.sh` gives a complete,
usable machine (shell, git, Neovim, tmux, mise tools, apps). Desktop tweaks,
bin scripts and secrets come in later releases.

### Added

#### Bootstrap

- `install.sh` (curl-able, safe to re-run): asks for sudo once
  (`omarchy-sudo-keepalive`), installs packages with `omarchy-pkg-add`,
  installs apps with Omarchy's own installers, clones to `~/Work/dots`,
  backs up conflicting files, stows packages, adds include lines to
  Omarchy's config files, installs tmux plugins and mise tools, merges
  LazyVim Extras, asks for the git name/email if unset, and ends with a
  drift check.
- Config pattern: Omarchy's config files stay real files; each gets one
  line that loads our stowed `<name>.dots[.ext]` file, which ends with an
  optional machine-local `.local` include (ignored by git).
- `omarchy` package: a post-update hook (`dots.hook`) that re-adds include
  lines and reports drift after every `omarchy update`; the same checks as
  `make doctor`. `lib/dots.sh` is the shared source of packages, include
  lines and checks.

#### Packages

- `bash`: `~/.bashrc.dots` with vi mode (Omarchy's inputrc re-applied to
  the vi keymaps, fzf `^R`/`^T` kept), a `[vim]` command-mode indicator,
  `ll`/`ln`/`mkdir`/`grep`/`path` aliases, and git completion for
  Omarchy's `g` alias. Starship stays Omarchy's.
- `git`: `config.dots` with `fetch.prune`, `zdiff3` conflicts and gh as
  the GitHub HTTPS credential helper; a lean global `~/.config/git/ignore`.
- `nvim` (on Omarchy's LazyVim): super-tab completion keys, grug-far
  hidden/ignored toggles, vim-tmux-navigator, and LazyVim Extras (sidekick
  plus language Extras).
- `tmux` (on Omarchy's tmux): prefix2 off, nvim-aware `Ctrl+h/j/k/l`
  pane movement, `prefix k` session picker (sessions + zoxide
  directories), `prefix =` synchronize panes, undercurl support, and
  resurrect + continuum at pinned commits (no TPM).
- `mise`: `conf.d/config-dots.toml` with `.ruby-version` support and
  global tools at `latest` (uv, go, terraform, aws-cli, ansible, yamllint,
  yarn, confluence-cli, jira-cli, todoist-cli).
- Extra packages: `wget`, `nmap`.

#### Apps

- 1Password and Chrome (default browser) via Omarchy's installers.
- ChatGPT desktop via `omarchy-install-ai-chatgpt`.
- Slack, Teams and Outlook web apps via `omarchy-webapp-install`; a
  launcher is recreated when its URL changes.

#### Docs and tooling

- `docs/decisions/`: every keep/drop decision and what Omarchy already
  provides; `docs/setup/omarchy.md`: the steps that can't be scripted;
  `docs/guides/`: troubleshooting ("it used to do X") and Ruby.
- README usage guide, `AGENTS.md` for AI agents, MIT license and this
  changelog.
- prek hooks (whitespace, toml/yaml, typos, committed, shellcheck),
  `make setup`, `make lint`, `make doctor`.

[Unreleased]: https://github.com/fernandoaleman/dots/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/fernandoaleman/dots/compare/v0.2.0...v1.0.0
[0.2.0]: https://github.com/fernandoaleman/dots/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/fernandoaleman/dots/releases/tag/v0.1.0
