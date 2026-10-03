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
`libyaml`, `lua` (`lua51`), `mysql-client` (Omarchy ships `mariadb-libs`),
`libpq`/`psql` (Omarchy ships `postgresql-libs`), `ripgrep`, `rsync`,
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
| `starship` | **Omarchy's `starship.toml` kept as is**; the old one dropped. Its `disabled = true` modules were redundant (Omarchy's explicit `format` only renders directory, git branch, git status, character); `git_status` off and blue directories dropped; its prompt symbols referenced an undefined `color_green`/`color_red` palette; its `vimcmd_*` symbols don't work in bash (starship: *"only supported in cmd, fish and zsh"*). The vi-mode indicator moved to readline instead (see [shell.md](shell.md)). Starship has no include mechanism, so nothing stowed (tally unchanged) |

## Still to review

- **Group A** (Omarchy installs it, the old repo had a config): `lazygit`, `lazydocker`, `mise`, `herdr`, `bat`. (`tmux-ssh` moves
  to the bin scripts section.)
- **Group D** (not installed): `act`, `bottom`, `colordiff`, `htop`,
  `markdownlint-cli2`, `ncdu`, `nmap`, `thefuck`, `todoist-cli`, `wget`,
  `yarn`, `rclone`, `awsvpnclient`, `espanso`, `alacritty`, `ghostty`, Slack,
  Microsoft Teams, ChatGPT, Loom, Raindrop.io, plus Omarchy's Discord/Zoom web
  apps and the `claude`/`codex`/`opencode` mise stubs.
- **mise tools** from the old `mise/config.toml`.
