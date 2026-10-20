# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
`1.0.0` marks Omarchy complete, including secrets via 1Password, verified
by a clean install; macOS support comes in later `1.x` releases.

## [Unreleased]

### Changed

- The Mac uses the Moonlight theme instead of OneDark "dark": Ghostty's
  Catppuccin Mocha at 0.9 opacity with blur, Moonlight's Neovim colors and
  tmux bar, and its fisherman background as the wallpaper (`install.sh`
  clones the theme repo and sets them).
- The docs refer to the employer as "work", and the git history was
  rewritten to match (all commit IDs before this release changed).
- Desk monitors at scale 1.25 instead of 1.6 (more room; tried since
  2026-10-18).
- Todoist is the native app (AUR `todoist-appimage`) instead of the web
  app, run with Chromium's sandbox on (not `--no-sandbox`) and its login
  token in the keyring (`--password-store=gnome-libsecret`), and it
  stays running in the tray when its window is closed.
- Slack is the native desktop app (AUR `slack-desktop`) instead of the
  Chrome web app: nicer notifications, which also keep coming with its
  window closed; `install.sh` removes the old web app.
- tmux sessions: sesh is back (mise tool, both systems): `prefix k` opens its
  built-in picker, `prefix L` is `sesh last`; the custom session picker is
  removed.
- tmux popups use the terminal's colors on tmux 3.8 too (the Mac's
  Homebrew tmux drew them as a grey box).
- Mac setup: move work-used 1Password logins (e.g. GitHub) to the company
  account before installing on the work laptop.

### Added

- `private words` git hook (prek): fails a commit or `make lint` when a
  staged file or the message contains a word from the machine-local list
  `~/.config/dots/private-words`; no list, no commit.
- Syncthing for coding agents (both systems): `~/.claude/projects`
  (Claude Code's sessions and memory, old path-named folders kept local)
  and `~/.agents` (instructions, team files, skills) on every machine, via
  Omarchy; Omarchy keeps old versions (trash can 30 days for sessions,
  staggered 365 days for `~/.agents`).
- One global instructions file for every coding agent,
  `~/.agents/AGENTS.md` (private, not in dots): `install.sh` creates a
  `~/.claude/CLAUDE.md` that imports it (with `team.md` and
  `personas.md`) and links Codex's, opencode's and pi's `AGENTS.md` to it
  (both systems).
- Git aliases from the old setup, for muscle memory, in their own
  section of `~/.config/bash/aliases` (to retire later): `gs`, `ggpur`, `gftf`, `ggpurt`, `grhh`, and `gcm` = `git switch master`
  (replaces Omarchy's `gcm`, `git commit -m`, which is `gcmsg` now).
- `gopls` and `lua-language-server` (both systems), for Claude Code's
  `gopls-lsp` and `lua-lsp` plugins.
- `claude-migrate-projects`: copies Claude Code's path-named project
  folders (sessions and memory) to one name per project, the repo name
  from its git remote or else the folder's name, for syncing across
  machines; reads a `.claude` folder or a backup, looks up paths and
  remotes on the machine they came from (`--host`, over ssh, read-only),
  and takes machine-local overrides from `~/.config/dots/claude-projects`.
  A dry run unless `--apply`, which backs up the target first, never
  changes the old folders, and is safe to re-run.
- `claude` wrapper (bash, both systems) with `claude-project-name`: starts
  Claude Code with `CLAUDE_CODE_PROJECT_DIR_NAME` set to the folder's repo
  name (from its git remote; else the repo's or folder's name), so sessions
  and memory get the same folder on every machine; the migration script
  uses the same naming. Off until `~/.claude/.claude.json` exists (then it
  also sets `CLAUDE_CONFIG_DIR`).
- Omarchy theme **Moonlight**: the Mac Studio's look (Ghostty's Catppuccin
  Mocha in foot, background at 0.9, Neovim in Macchiato) with six night-sky
  backgrounds (Unsplash illustrations; cycle with the background switcher).
  Published as its own repo,
  [omarchy-moonlight-theme](https://github.com/fernandoaleman/omarchy-moonlight-theme);
  `install.sh` clones it into `~/Work` and links it.
- Moonlight's tmux bar: the Mac Studio's Macchiato bar, only while
  that theme is active (other themes keep Omarchy's), kept in the tmux
  package (`themes/moonlight.conf`) rather than the theme; a theme-set hook
  reloads tmux on every theme switch.
- Omarchy theme **One Dark** (`themes/one-dark`): onedark.nvim's "dark"
  palette (as on the Mac) in `colors.toml`, onedark.nvim in Neovim, Yaru-blue
  icons; `install.sh` links every theme in `themes/` and downloads its
  background (Unsplash, Pawel Czerwinski).
  Apply with `omarchy theme set "One Dark"`.
- sesh's picker shows a preview of the highlighted session beside the list
  (new `sesh` package with `sesh.toml`; folders listed with eza, like
  Omarchy's `ls`); the `prefix k` popup is wider
  (80%x70%) to make room.
- macOS: `install.sh` installs Rosetta 2 (Intel-only tools such as
  wkhtmltopdf).
- `NEW_RELIC_API_KEY` from 1Password (`dots/token`) for New Relic's API
  (NerdGraph), next to the New Relic MCP's browser sign-in.
- macOS: Chrome stops offering to save passwords (`PasswordManagerEnabled`
  policy, recommended level); 1Password saves them.
- `cache-sesh-dirs` adds `~/Work`'s projects to zoxide for the sesh picker;
  `install.sh` runs it.

### Fixed

- Super+V/Super+C paste and copy in the tmux window (Super+Alt+Return)
  again: its `tmux` app-id hid it from Omarchy's terminal detection, so
  they sent Ctrl+V (`^V` in Neovim) and Ctrl+C (interrupt).
- Switching Omarchy themes no longer piles up duplicate tmux
  `terminal-overrides`/`terminal-features` entries (the theme-set hook resets
  them before reloading).
- Moonlight: the whole terminal is see-through, not only foot's
  padding (`alpha-mode=matching`; Omarchy paints tmux's pane background in
  the theme color, which foot kept solid).
- macOS: `install.sh` installs the Todoist CLI's agent skills on the Mac too
  (the step ran on Omarchy only).

### Removed

- macOS: `try` dropped from the Brewfile (unused; its Homebrew Ruby came
  before mise's Ruby in non-interactive shells).

## [1.2.0] - 2026-10-12

macOS: the same `install.sh` sets up a Mac, work or personal, to match
Omarchy (bash, Neovim, tmux, mise, Docker, secrets, settings); GitHub over
HTTPS everywhere; Obsidian vaults synced with Syncthing.

### Added

- `pandoc`, `mkcert` (Omarchy packages) and `just` (mise tool).
- Bazecor (Dygma keyboard configurator) from the AUR.
- Voxtype dictation via Omarchy's installer.
- Todoist web app on Omarchy.
- Syncthing on Omarchy (Obsidian vaults; ports open to the LAN and Tailscale
  only).
- Machine profile (`personal` or `work`), asked on the first run and kept in
  `~/.config/dots/profile`; `DOTS_PROFILE` overrides.
- Secrets on macOS, and 1Password per machine profile: work items live in
  the work account; the personal profile reads both accounts, the work
  profile only the work account.
- GitHub over HTTPS on every machine: `gh-role-login` signs `gh` in as the
  profile allows (work: a company-only fine-grained token from 1Password;
  personal: the browser), and SSH GitHub URLs are rewritten to HTTPS.
- SSH keys split: a new personal `id_ed25519` for your own machines; the
  old one kept only as a work key; no SSH key on GitHub.
- Git identity: asked once if missing (now on macOS too), with dots' name
  and email pre-filled, and used as is without a terminal.
- Syncthing on macOS starts at login (Homebrew service); vaults in
  `~/Obsidian`, Omarchy as the hub, versioning on Omarchy and the Mac
  Studio. `install.sh` sets up the vault folders on both systems, and on a
  Mac also its name and the pairing with Omarchy.
- macOS, first stage: the same `install.sh` one-liner bootstraps a fresh Mac
  (Homebrew, bash 5, git, gum, stow, jq, then dots) and installs the
  `Brewfile` (packages and apps; personal-only entries by profile), with
  Chrome as the default browser.
- macOS bash: Omarchy's bash defaults (downloaded at a pinned tag) plus a
  Mac add-on (`macos` stow package), with Homebrew's bash 5 as the login
  shell.
- macOS config files: Omarchy's templates (git, tmux, starship, lazygit,
  btop, herdr, Ghostty, opencode) seeded from the pinned download, dots' git
  and tmux settings and tmux plugins, Ghostty with Option as Alt, and tmux's
  `prefix ?` keybindings help.
- Ghostty on macOS: OneDark "dark" theme, generated from onedark.nvim's
  palette.
- Neovim on macOS: Omarchy's LazyVim config rebuilt from the omarchy-nvim
  package's sources at pinned commits, dots' `nvim` package and LazyVim
  Extras, OneDark "dark" as the colorscheme, and the macOS clipboard
  (`pbcopy`/`pbpaste`) outside SSH.
- mise on macOS: Omarchy's setup (no auto-prune, `~/Work/.mise.toml`,
  global Node, first-run launchers for claude, codex, gh and opencode) and
  dots' tools from `config-dots.toml`.
- Docker on macOS: Colima (4 CPUs, 8 GiB, starts at login) with Omarchy's
  log rotation and dots' networks off 172.17, and `docker compose`/`buildx`
  found by Docker.
- SSH on macOS: dots' `~/.ssh/config` and Omarchy's keepalive settings
  (its own script); Colima no longer edits `~/.ssh/config`.
- macOS: tmux-ssh is installed too (`install.sh`).
- Tailscale on macOS: `install.sh` turns Use Tailscale DNS off (it breaks
  the AWS VPN's DNS there), and a `tailscale` command (alias).
- macOS: Tailscale added to Login Items (its own checkbox can fail).
- macOS settings: Dock auto-hide, Finder (extensions, path bar, home
  folder), expanded Save dialogs, fast key repeat, text substitutions off,
  TextEdit plain text and no `.DS_Store` on network shares, each verified
  on macOS 27.
- macOS: Moonlight captures system keyboard shortcuts in fullscreen.
- Voxtype on macOS: Omarchy's config, the `base.en` model and Voxtype.app
  in Login Items; record from its menu bar.
- macOS: a HostName from the model (`mac-studio`, `macbook-pro`) when unset.
- macOS: `~/.hushlogin` (no "Last login" line in new shells).
- macOS: `install.sh` ends with the dots check (`dots_doctor`), which also
  checks Colima's Docker networks.

### Fixed

- `aws-role-login` finds the MFA item when the 1Password app has more than
  one account.
- `aws-role-login` on macOS shows the session's expiry as a local time.
- `install.sh` no longer stops at the git identity prompt when run without
  a terminal (e.g. over `ssh host …`).
- `make doctor` on macOS no longer reports every file as unlinked.
- macOS checklist: the AWS VPN signs in with the AWS SSO login (now in the
  work account), not AWS Role Switch.

## [1.1.0] - 2026-10-10

Remote desktop: use the Omarchy desktop from a Mac with Moonlight, at home
or away over Tailscale.

### Added

- Remote desktop host: Sunshine (Moonlight streaming over the LAN and
  Tailscale), installed as Omarchy's installer does it, with its open bugs
  fixed (real unit name, no double start, no browser-wide certificate
  bypass). It streams the middle desk monitor (`output_name = DP-2`) and
  falls back to a connected one.
- `docs/setup/macos.md`: the Mac checklist (Tailscale DNS off, AWS VPN,
  Moonlight and its keyboard capture, terminal, small settings).
- Troubleshooting for Sunshine, Moonlight, and Tailscale with the AWS VPN
  on macOS.

### Changed

- `~/.ssh/config`: `omarchy` and `mac-studio` use their Tailscale addresses
  (reachable at home and away).

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

[Unreleased]: https://github.com/fernandoaleman/dots/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/fernandoaleman/dots/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/fernandoaleman/dots/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/fernandoaleman/dots/compare/v0.2.0...v1.0.0
[0.2.0]: https://github.com/fernandoaleman/dots/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/fernandoaleman/dots/releases/tag/v0.1.0
