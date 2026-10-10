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
