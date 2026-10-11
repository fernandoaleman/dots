# macOS

Started 2026-10-10 (in progress). The Mac phase (dots 1.x): the 16" M3
MacBook Pro, clean macOS 27 install, as a **full mirror** of the Omarchy
setup, so work continues locally if the remote desktop into Omarchy is
blocked (e.g. office IT blocking ports or Tailscale). Manual steps:
[setup/macos.md](../setup/macos.md).

## One `install.sh` for both systems

Decided 2026-10-10: the same curl one-liner on any machine. `install.sh`
detects macOS vs Omarchy; shared steps (stow, secrets from 1Password, tmux
plugins, mise, git identity, doctor) run on both, and only system-specific
steps branch (packages and apps: Omarchy's scripts vs Homebrew; Omarchy-only
steps such as Sunshine, Docker networks, sudoless Docker).

## Machine profiles: personal or work

Decided 2026-10-10. Besides the OS, machines differ by owner: the Omarchy
desktop and the Mac Studio are **personal**; the MacBook Pro is a **work**
laptop (company-owned, with IT's RMM agent, NinjaOne: per its docs Full Disk
Access, Accessibility and Screen Recording, plus admin scripting, so assume
IT can read files, see the screen and run anything). One `install.sh`; the
first run asks "personal or work?" (gum, with a plain `read` fallback) and
remembers it in machine-local `~/.config/dots/profile` (never committed);
`DOTS_PROFILE=work install.sh` overrides. The profile selects apps/packages
and secrets. Secrets follow the **1Password account**: work items move to
the user's work 1Password account; the work profile reads only that
account (the personal account never needs signing in there), the personal
profile reads both. Details are decided step by step while building.

## Bootstrap on a fresh Mac

Decided 2026-10-10. The same one-liner as on Omarchy; nothing installed by
hand first. A fresh macOS has `curl` and bash 3.2 (`/bin/bash`), and only a
`git` placeholder (real git needs Apple's Command Line Tools). So the top of
`install.sh` runs under bash 3.2 (no bash 4+ features) and, on macOS:

1. installs Homebrew with its official installer
   (`NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`;
   one password prompt; it also installs the Command Line Tools, giving
   git), then `eval "$(/opt/homebrew/bin/brew shellenv)"`;
2. `brew install bash git gum stow jq`;
3. clones or updates `~/Work/dots`, then re-runs the repo's `install.sh`
   with Homebrew's bash 5 (`bash <(curl …)` leaves no file to re-run);
4. asks the profile question, then continues (Brewfile, configs, secrets).

macOS has no `omarchy-sudo-keepalive`: the Mac branch asks for the password
once and refreshes it in the background. Omarchy skips the bootstrap
(already bash 5 with gum).

## Homebrew packages: a `Brewfile`

Decided 2026-10-10. A `Brewfile` at the repo root (not a stow package, so
never linked into `$HOME`; Omarchy never reads it), used only by the macOS
branch: `brew bundle --file="$DOTS_DIR/Brewfile" --no-upgrade`. Per
Homebrew's docs: `brew bundle` *"will attempt to upgrade all software"* by
default, hence `--no-upgrade` (re-runs only add what's missing; updates
stay a deliberate `brew upgrade`); it removes nothing unless `brew bundle
cleanup` runs (never from `install.sh`); Brewfiles are Ruby, so profile
differences are conditions on `ENV["DOTS_PROFILE"]`; Mac App Store (`mas`)
apps need a signed-in Apple ID, so they're personal-profile only.

### Brewfile section 1: command-line tools

Decided 2026-10-10, same for both profiles:

- **Bootstrap** (installed first): `bash`, `git`, `gum`, `stow`, `jq`.
- **Mirroring Omarchy's tools** (its package list; its bash defaults use
  several): `bash-completion@2`, `bat`, `btop`, `dua-cli`, `eza`,
  `fastfetch`, `fd`, `fzf`, `herdr`, `imagemagick`, `lazydocker`,
  `lazygit`, `libyaml`, `mise`, `neovim`, `ripgrep`, `starship`, `tlrc`
  (Homebrew's `tldr` is the outdated client; `tlrc` is the official one,
  same `tldr` command), `tmux`, `tree-sitter-cli`, `try` (tobi/try, as
  Omarchy's `tobi-try`), `usage`, `zoxide`.
- **Ours, as on Omarchy:** `wget`, `nmap`, `shellcheck`, `mysql-client` and
  `libpq` with `link: true` (both keg-only; `brew bundle` unlinks keg-only
  formulas otherwise), for `mysql`/`mysqldump`/`mysql_config` and
  `psql`/`pg_dump`.
- Left out: Omarchy's Linux-desktop packages; `lua@5.1` (gone from
  Homebrew) and `luarocks` (Neovim bundles LuaJIT; LazyVim doesn't need
  rocks); Docker (apps section).
- **mise:** our stowed `config-dots.toml`, plus Omarchy's global tools
  (`claude`, `codex`, `gh`, `node`, `prek`) via `mise use -g`, plus Omarchy's
  `~/Work/.mise.toml` (adds a project's `bin/` to PATH).

### Apps: Homebrew first, App Store last

Decided 2026-10-10: install apps with a Homebrew cask (or the vendor's
direct download) whenever one exists; the Mac App Store only when there's
no other way (no personal Apple ID on the work laptop, and avoided
elsewhere too). Apps are reviewed one at a time.

### App reviews

Inventory taken from the Mac Studio (2026-10-10, over SSH): 62 formulae
installed on request (`brew leaves --installed-on-request`), 36 casks, 10
App Store apps, plus a few apps from outside Homebrew (Microsoft Office,
Brave, Helium, TablePlus, Bazecor, Pixelmator Pro, TickTick).

| App | Install | Profiles | Notes |
|---|---|---|---|
| 1Password + CLI | casks `1password`, `1password-cli` | both | work profile: sign in to the work account only. "1Password for Safari" (App Store) dropped: 1Password 8's Safari extension is believed to ship in the app (verify on the fresh Mac); Chrome's extension comes from the Chrome Web Store |
| Google Chrome | cask `google-chrome` | both | the only browser (Brave, Helium dropped). Default browser: `install.sh` runs `defaultbrowser chrome` (formula `defaultbrowser`); macOS asks once to confirm |
| Tailscale | cask `tailscale-app` only | both | not the `tailscale` formula too (two Tailscale setups on one Mac). The app ships the CLI: try `tailscale set --accept-dns=false` from `install.sh` on the fresh Mac; checklist step until then |
| Moonlight | cask `moonlight` | both | updates via `brew upgrade` (cask isn't self-updating). Try setting "Capture system keyboard shortcuts: in fullscreen" with `defaults write` on the fresh Mac; checklist step until then |
| AWS VPN Client | cask `aws-vpn-client` | both | profile imported by hand (checklist). `aws-vpn-cli` (third-party `jlars22/tools` wrapper; needs the official app; optional sudoers change) dropped |
| Ghostty | cask `ghostty` + `font-jetbrains-mono-nerd-font` | both | mirrors Omarchy's terminal: Omarchy's `config/ghostty/config` (v4.0.4) stays the real `~/.config/ghostty/config` with an include line for our `config.dots` (Ghostty's optional `config-file = ?path`): `macos-option-as-alt = true` (tmux Alt keys), a larger font (start at 13; Omarchy's 9 suits its 1.6 scaling), Linux-only `async-backend = epoll` neutralized. Colors: **OneDark "dark"** (user's choice): a Ghostty theme `onedark-dark` generated from [onedark.nvim](https://github.com/navarasu/onedark.nvim)'s `palette.lua` (`dark` style) and its `terminal.lua` color mapping, kept in dots; `config.dots` sets `theme = onedark-dark` (Omarchy's optional theme include finds nothing on a Mac) |
| Raycast | dropped | n/a | moving to Omarchy (Pro not renewed); macOS 27's Spotlight has clipboard history, so Spotlight keeps Cmd+Space. Quicklinks: find a replacement later ([TODO](../TODO.md)) |
| Slack | cask `slack` (desktop app) | both | on a Mac the native app is better than Omarchy's web app: drawing on a shared screen in huddles works only in the Mac/Windows desktop app |
| Microsoft Teams | Chrome web app (manual: "Install page as app") | both | like Omarchy (`teams.cloud.microsoft`); enough for occasional chats and calls (chat, audio/video, screen sharing in Chrome); the native app (cask `microsoft-teams`) was first chosen, then replaced. Chrome creates real apps in `~/Applications/Chrome Apps/` (own window, Dock icon, found by Spotlight); macOS has no script for it (Chrome's force-install policy needs an MDM), so it's a checklist step |
| Outlook | Chrome web app (manual) | both | like Omarchy (`outlook.office.com`) |
| Word, Excel, PowerPoint | not installed | n/a | Office on the web when needed; casks `microsoft-word` / `-excel` / `-powerpoint` exist if ever necessary |
| Zoom | cask `zoom` (native app) | both | used for work meetings; Zoom's web client is the weakest (pushes to the app); Omarchy keeps its built-in Zoom web app |
| Docker | formulas `colima`, `docker`, `docker-compose`, `docker-buildx` | both | CLI only (the GUI is never used; the company's Docker license stays available): Colima runs Docker Engine in a VM; `docker compose`/`buildx` as CLI plugins (`cliPluginsExtraDirs` in `~/.docker/config.json`); lazydocker via Colima's Docker context or `DOCKER_HOST` (verify). Settings: `brew services start colima` (at login); VM 4 CPUs / 8 GB RAM; networks off `172.17` like Omarchy (bridge `172.31.0.1/16`, pools `192.168.128.0/17`) in `colima.yaml` (`docker:` section), because the VM's default bridge would capture container traffic to work servers on `172.17.x` |
| Claude Code, Codex, opencode | mise (global), not casks | both | like Omarchy: `mise use -g` with Omarchy's `claude`, `codex` (+ `gh`, `node`, `prek`) plus `opencode`; casks `claude-code`/`codex` dropped (two copies would fight over the command) |
| ChatGPT (with Codex) | cask `chatgpt` (native) | both | work-paid; mirrors Omarchy's native `openai-codex-desktop` |
| Claude desktop | cask `claude` (native) | both | work-paid (Omarchy has no Claude desktop app) |
| Gemini desktop | dropped | n/a | personal account; browser (or a Chrome web app) when needed |
| Discord | Chrome web app (manual) | personal only | personal account: Mac Studio only, as a web app like Omarchy's built-in one; not on the work laptop |
