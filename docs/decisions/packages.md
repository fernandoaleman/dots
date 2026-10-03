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

## Still to review

- **Group A** (Omarchy installs it, the old repo had a config): `git`
  (+ `git-delta`), `neovim` (the separate `fernandoaleman/nvim` repo), `tmux`
  (+ `gitmux`, `sesh`, TPM, `tmux-ssh`), `starship`, `btop`, `lazygit`,
  `lazydocker`, `mise`, `herdr`, `bat`.
- **Group D** (not installed): `act`, `bottom`, `colordiff`, `htop`,
  `markdownlint-cli2`, `ncdu`, `nmap`, `thefuck`, `todoist-cli`, `wget`,
  `yarn`, `rclone`, `awsvpnclient`, `espanso`, `alacritty`, `ghostty`, Slack,
  Microsoft Teams, ChatGPT, Loom, Raindrop.io, plus Omarchy's Discord/Zoom web
  apps and the `claude`/`codex`/`opencode` mise stubs.
- **mise tools** from the old `mise/config.toml`.
