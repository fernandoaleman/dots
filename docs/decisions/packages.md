# Packages

Started 2026-10-02. Source lists: the old repo's
`dotfiles/.chezmoidata/packages.toml` (both the macOS and Arch sections),
compared against Omarchy's `/usr/share/omarchy/install/omarchy-base.packages`
and what is installed.

## How packages are installed

Always through Omarchy's own scripts in `/usr/share/omarchy/bin`, never raw
`pacman -S` / `yay -S`:

| Script | What it does |
|---|---|
| `omarchy-pkg-add <pkgs…>` | `sudo pacman -S --noconfirm --needed` only if something is missing, then verifies each package with `pacman -Q` |
| `omarchy-pkg-aur-add <pkgs…>` | the same through `yay` for AUR packages |
| `omarchy-pkg-present` / `omarchy-pkg-missing` | true if all present / any missing (used as guards) |
| `omarchy-sudo-keepalive` | prompt for sudo once and keep it alive for the run (sourced by `install.sh`) |

When Omarchy has a dedicated installer for an app (what its menu runs),
`install.sh` uses that instead, guarded so re-runs skip it.

## Skipped: macOS-only

`coreutils`, `mas`, `reattach-to-user-namespace`, `switchaudio-osx`,
`appcleaner`, `macdown`, `macx-youtube-downloader`, `qlmarkdown`, `raycast`,
`reflex-app`, `docker-desktop` (Docker runs natively on Linux) and all Mac
App Store apps (1Password for Safari, AdGuard Mini, Amphetamine, AWS Extend
Switch Roles, Keynote, Numbers, Pages, SonicWall Mobile Connect).

## Settled by earlier decisions

| Package | Why |
|---|---|
| `zsh`, `zsh-autosuggestions`, `zsh-completions`, `zsh-syntax-highlighting` | bash everywhere (see [shell.md](shell.md)) |
| `chezmoi` | replaced by GNU Stow |
| `shellcheck` | dev tool for this repo; `make setup` installs it with `omarchy-pkg-add` |
| `age` | only used for chezmoi encryption; to be confirmed in the secrets section (1Password replaces it) |

## Already installed by Omarchy, no old config (no action)

Listed in `omarchy-base.packages` or present on a fresh install:
`bash`, `curl`, `eza`, `fastfetch`, `fd`, `fzf`, `gum`, `imagemagick`, `jq`,
`libyaml`, `lua` (`lua51`), `mysql-client` (Omarchy ships `mariadb-libs`: `mysql_config`/`libmariadb` for the `mysql2` gem; the `mysql`/`mysqldump` commands come from `mariadb-clients`, added to `PACMAN_PACKAGES` 2026-10-04),
`libpq`/`psql` (Omarchy ships `postgresql-libs` 18: `libpq`, `pg_config`, `psql`, `pg_dump`, `pg_restore`; newer clients work with the PostgreSQL 16 servers), `ripgrep`, `rsync`,
`tldr`, `zoxide`, `docker`, JetBrains Mono Nerd font
(`ttf-jetbrains-mono-nerd-basic`), `obsidian`, `gnupg`.

## Installed by `install.sh` with Omarchy's installers

Omarchy does not install these by default, but its menu has installers for
them. `install.sh` runs the same commands, each guarded:

| App | Omarchy menu | Command | Guard |
|---|---|---|---|
| 1Password + `op` CLI (+ Chromium extension) | Install → Service → 1Password | `omarchy-install-service-1password` | `omarchy-pkg-missing 1password 1password-cli` |
| Google Chrome (AUR `google-chrome`; also sets up browser policy, flags and theme) | Install → Browser → Chrome | `omarchy-install-browser chrome` | `omarchy-pkg-missing google-chrome` |
| Chrome as default browser (Omarchy's default is Chromium) | Setup → Defaults → Browser → Chrome | `omarchy-default-browser chrome` | current value isn't `chrome` |

## Group A: Omarchy installs it, the old repo had a config

| Package | Decision |
|---|---|
| `git` | Omarchy's config kept, plus a small `config.dots` (see [git.md](git.md)) |
| `git-delta` | Dropped, not installed |
| `neovim` | Omarchy's LazyVim plus `.dots` plugin files and Extras (see [nvim.md](nvim.md)); the separate `fernandoaleman/nvim` repo is retired (archived later, in the Mac phase) |
| `tmux` | Omarchy's config plus `tmux.dots.conf` (see [tmux.md](tmux.md)) |
| `sesh` | Dropped, replaced by the `prefix k` picker |
| TPM | Dropped: resurrect + continuum are pinned clones |
| `gitmux` | Dropped: it was never used in the status bar |
| `btop` | **Omarchy's `btop.conf` kept as is**. Only 3 real differences from the old file (the rest was `True`/`true` casing and newer btop 1.4.7 options): `color_theme` `"Default"` → Omarchy's `"current"` (follows the Omarchy theme; `omarchy-theme-set` restarts btop), `theme_background` false → true, `vim_keys` false → **true**. btop has no include mechanism and rewrites its config on exit (`save_config_on_exit`), so nothing is stowed; tweak it through btop's options menu (`o`) |
| `lazygit` | **Omarchy's empty `config.yml` kept** (lazygit v0.65.0 defaults). Old file: `showFileTree`, `quit: q` already defaults; `editCommand`/`editCommandTemplate` outdated (lazygit uses `$EDITOR`, Omarchy's nvim launcher); `showIcons` no longer exists (icons are now `nerdFontsVersion`, default off). Dropped the real differences too: icons, line staging (`useHunkModeInStagingView` now defaults to true), skipping the discard confirmation |
| `lazydocker` | Dropped: the old config was empty, Omarchy ships none, and Omarchy runs lazydocker as root via `pkexec` (Super+Shift+D), so a user config wouldn't apply anyway |
| `mise` (settings) | `~/.config/mise/conf.d/config-dots.toml` (hyphen: mise reads dotted conf.d names as environments): **kept `idiomatic_version_file_enable_tools = ["ruby"]`** (default `[]`, so mise would ignore `.ruby-version`). Dropped `legacy_version_file` (already `true` by default) and **`ruby.compile = false`**, which would make Ruby 2.7.8 uninstallable (mise's precompiled `jdx/ruby` has no 2.7.x builds; unset = precompiled first, else compile). Omarchy's `config.toml` (written by `mise use -g` and Omarchy's AI/gh launchers) is never stowed |
| mise **tools** | All `latest` in `config-dots.toml` (every `omarchy update` runs `mise up`, so they stay current; projects pin their own versions): `uv` (replaces `pipx`; mise's `pypi:` backend uses uv), `go`, `terraform`, `aws-cli`, `ansible` (pypi; exposes `ansible-core`'s commands), `yamllint`, `yarn`, `npm:confluence-cli`, `github:ankitpokhrel/jira-cli` (`exe = "jira"`). **Dropped as globals:** `node` (Omarchy installs it, and its `config.toml` wins over conf.d), `ruby` (system 3.4.10 + per-project `.ruby-version`), `python` (system 3.14.7; uv brings its own for tools). `install.sh` runs `mise install --yes`. All 9 verified installing and resolving |
| `herdr` | **Omarchy's config and functions kept**; all old items dropped on Omarchy. The old `herdr/config.toml` was a copy of Omarchy's (only `onboarding = false` differed, and it lacked Omarchy's newer `alt+enter` split). `herdr-layouts.sh` + `hdl`/`hds`/`hdlm`/`hsl` symlinks were a macOS port of Omarchy's `fns/herdr`; `open-herdr.sh` (Raycast) and the integration setup were Mac-only. **Mac phase:** reuse those ports from the old repo (`dotfiles/dot_config/herdr/`, `dot_local/bin/executable_open-herdr.sh`, `.chezmoiscripts/run_onchange_after_37-setup-herdr.sh.tmpl`); the layouts script is bash-3.2 compatible |
| `bat` | **Omarchy's setup kept** (`BAT_THEME=ansi` in `default/bash/envs`: bat, man pages and fzf previews follow the terminal theme). The old `bat/themes/tokyonight_night.tmTheme` was dropped: it was never active (no `BAT_THEME`, no bat config, no `bat cache --build`) |
| `starship` | **Omarchy's `starship.toml` kept as is**; the old one dropped. Its `disabled = true` modules were redundant (Omarchy's explicit `format` only renders directory, git branch, git status, character); `git_status` off and blue directories dropped; its prompt symbols referenced an undefined `color_green`/`color_red` palette; its `vimcmd_*` symbols don't work in bash (starship: *"only supported in cmd, fish and zsh"*). The vi-mode indicator moved to readline instead (see [shell.md](shell.md)). Starship has no include mechanism, so nothing stowed (tally unchanged) |

## Group D: not installed by Omarchy

### Command-line utilities

| Tool | Decision |
|---|---|
| `wget`, `nmap` | **Added**: official repos, installed by `install.sh` via `omarchy-pkg-add` (`PACMAN_PACKAGES`) |
| `todoist-cli` (`td`) | **Added** as a mise tool, `npm:@doist/todoist-cli` (not in Arch repos/AUR). The old Todoist "skills" setup was Mac-only; `td-login` → bin scripts section |
| `bottom`, `htop` | Dropped: Omarchy's **btop** |
| `ncdu` | Dropped: Omarchy's **dua** (`dua i`) |
| `colordiff` | Dropped: `diff --color`, colored `git diff` |
| `markdownlint-cli2` | Dropped: the LazyVim markdown Extra installs it via Mason for nvim |
| `thefuck` | Dropped (last release 3.32, Jan 2022); its `settings.py` too |
| `act` | Dropped |
| `hunk` | Dropped (2026-10-04): [hunk](https://github.com/modem-dev/hunk), a *"review-first terminal diff viewer for agentic coders"* (review an AI agent's changes file by file, hunk by hunk). Omarchy's **lazygit** covers diff review (per-hunk/line staging and discarding). On Arch only in the AUR (`hunk-bin`, prebuilt); was a recent Homebrew install on the Mac. To add: `omarchy-pkg-aur-add hunk-bin` in `install.sh` |

### Terminals

| Item | Decision |
|---|---|
| Terminal | **foot, Omarchy's default** (decided 2026-10-03). Omarchy's foot/ghostty/alacritty configs already cover the old personal additions: Shift+Enter (`CSI 13;2u`, Claude Code), Super+C/V via `shift/control+insert`, theme include, padding 14, JetBrains Mono Nerd Font |
| Personal tweaks | Dropped; Omarchy's kept: font size 9 (old 12), Regular (old Medium), no copy-on-select, no hide-mouse-while-typing; zsh shell dropped |
| `alacritty`, `ghostty` | Not installed. Old configs (Omarchy-mirroring + Mac variants) stay in the old repo |
| **Mac** | foot is Linux-only, so the Mac needs another terminal, most likely **ghostty** with Option-as-Alt for Omarchy's tmux Alt keys. If the two machines feel inconsistent, switch Omarchy to ghostty too: `omarchy-install-terminal ghostty` (installs it and makes it the default; add it to `install.sh`) so both run the same terminal and Omarchy config |

### Apps

| App | Decision |
|---|---|
| **Slack** | **Web app** `https://1000bulbs.slack.com` (the workspace URL) via `omarchy-webapp-install` in `install.sh`, with an explicit icon (dashboard-icons `slack.png`). The generic `https://app.slack.com` was tried first but loops: with two workspaces it shows a picker whose *Launch* link leaves the app window for a browser tab, which then tries the desktop app's `slack://` link ("Open With… No Apps available"). The workspace URL skips the picker. Omarchy's icon auto-fetch fails for workspace subdomains, hence the explicit icon. You sign in once; the shared Chrome profile keeps it. Researched 2026-10-03: **drawing on shared screens is Mac/Windows desktop-app only**, so it's not possible on Linux with any client, including AUR `slack-desktop` (still *"beta"*, huddle screen share crashed until 4.51.180, July 2026). Huddles and screen sharing are supported in *"Google Chrome (… Linux)"*. Web-app notifications are normal system notifications (Omarchy's `quickshell` notification server), but only while the window is open: **keep it parked on a workspace**. The `bottelet.slack` Omarchy plugin is messaging-only (no huddles) and needs a personal Slack app token. Drawing sessions → use the Mac |
| **Teams**, **Outlook** | **Web apps** (`teams.cloud.microsoft`, `outlook.office.com`); Microsoft has no Linux Teams app |
| **ChatGPT** | **ChatGPT desktop** via Omarchy's installer `omarchy-install-ai-chatgpt` (package `openai-codex-desktop` from Omarchy's repo: *"Official ChatGPT desktop app with Codex"*), guarded like the menu: `omarchy-pkg-missing openai-codex-desktop` |
| **Spotify** | Omarchy's installer `omarchy-install-service-spotify` (`omarchy-pkg-add spotify`), guarded like the menu: `omarchy-pkg-missing spotify` |
| Discord, Zoom | Omarchy's preinstalled web apps |
| `claude`/`codex`/`opencode`… | Omarchy's mise launchers |
| Apple Music, iCloud, Claude (old web apps), Loom, Raindrop.io | Dropped for now (re-add a line to `WEBAPPS` in `install.sh`) |

`WEBAPPS` entries are `"Name|URL|icon URL"`: without an icon URL, `omarchy-webapp-install` fetches the site's own icon (the old PNG icon files weren't kept). `install.sh` recreates a launcher whose URL changed. Making Slack open on a fixed workspace automatically is a **Desktop section** item.

## Still to review

- **Group A**: done. (`tmux-ssh` moves to the bin scripts
  section.)
- **Group D** (not installed): `yarn` (done: mise), `rclone` (not set up; see [rclone guide](../guides/rclone.md)), `awsvpnclient` (not via dots; see [desktop.md](desktop.md)), `espanso` (dropped; see [desktop.md](desktop.md)).
