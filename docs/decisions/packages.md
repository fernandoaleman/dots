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

## Fresh install: Omarchy's update first

On a brand-new Omarchy install pacman has no package lists yet
(`/var/lib/pacman/sync/core.db` missing; Omarchy shows an "Update System"
notice), so `omarchy-pkg-add` (a plain `pacman -S --needed`) finds
nothing. Found in the clean-install VM test (2026-10-04). `install.sh`
then runs `omarchy-update -y` (unattended: Omarchy's full update, with
snapshot, migrations, AUR and mise) before installing anything, never a
bare `pacman -Sy` (an Arch partial upgrade). Its final reboot question
(kernel/Hyprland updated) should be answered No until `install.sh` is
done.

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
| `sesh` | Dropped 2026-10-03, back 2026-10-12 via mise ([tmux.md](tmux.md#sesh-back-2026-10-12)) |
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
| `pandoc` (`pandoc-cli`), `mkcert` | **Added** 2026-10-10 (`PACMAN_PACKAGES`; kept from the Mac's tools). `mkcert` uses Omarchy's `nss` so Chrome trusts its local certificates |
| `gopls`, `lua-language-server` | **Added** 2026-10-19 (`PACMAN_PACKAGES`, Brewfile): the language servers for Claude Code's official `gopls-lsp` and `lua-lsp` plugins, which run them from `PATH`. Neovim keeps its own copies through Mason (`~/.local/share/nvim/mason/bin`, not on `PATH`); the user chose separate installs over adding Mason's folder to `PATH` |
| `just` | **Added** 2026-10-10 as a mise tool (`config-dots.toml`), the same on every machine |
| `todoist-cli` (`td`) | **Added** as a mise tool, `npm:@doist/todoist-cli` (not in Arch repos/AUR). The old Todoist "skills" setup was Mac-only; `td-login` → bin scripts section |
| `bottom`, `htop` | Dropped: Omarchy's **btop** |
| `ncdu` | Dropped: Omarchy's **dua** (`dua i`). For muscle memory, an `ncdu` shell function (in `bash/.config/bash/aliases`) prints the matching `dua i <path>` command and returns 127 |
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
| **Slack** | **Native desktop app** (AUR `slack-desktop`, Slack's Linux build, still *"beta"*) via `omarchy-pkg-aur-add` in `install.sh`, since 2026-10-17; `omarchy update` keeps it current (`omarchy-update-aur-pkgs`). Was the **web app** of the workspace URL (2026-10-03): its notifications were system notifications too (Omarchy's notification server) but only while the window was open, and the user preferred the native app's (tried side by side). The native app keeps notifying from the tray when its window is closed and opens `slack://` links (the web app's sign-in loop: troubleshooting). Huddles: *"available on the Slack desktop and mobile apps, Google Chrome (Mac/Windows/Linux)"* (Slack help, *Use huddles in Slack*); the Linux app's huddle screen-share crash was fixed in 4.51.180 (July 2026; installed 4.52.178). Runs as a native Wayland window. Needs no keyring flag (checked 2026-10-17): it already keeps its key in gnome-keyring ("Slack Safe Storage") and its cookies are keyring-encrypted (`v11`). Cons: beta, community-packaged (AUR), its own Electron (more memory), sign-in in `~/.config/Slack` rather than Chrome's profile. **Drawing on shared screens is Mac/Windows desktop-app only** (*"Drawing on screens is currently supported on the Slack desktop app for Mac and Windows"*), so not on Linux with any client → use the Mac. `install.sh` removes the old web app where it exists (`omarchy-webapp-remove Slack`). The `bottelet.slack` Omarchy plugin is messaging-only (no huddles) and needs a personal Slack app token |
| **Teams**, **Outlook** | **Web apps** (`teams.cloud.microsoft`, `outlook.office.com`); Microsoft has no Linux Teams app. **teams-for-linux** (AUR `teams-for-linux-bin`, an unofficial desktop wrapper of the web app) was tried 2026-10-17..19 and **dropped**: in half of the right monitor its sidebar menu only showed full screen, while the web app shows the menu and the chat side by side; and with both running every message notified twice. Losing its tray icon is fine (Slack is the main messaging app) |
| **ChatGPT** | **ChatGPT desktop** via Omarchy's installer `omarchy-install-ai-chatgpt` (package `openai-codex-desktop` from Omarchy's repo: *"Official ChatGPT desktop app with Codex"*), guarded like the menu: `omarchy-pkg-missing openai-codex-desktop` |
| **Spotify** | Omarchy's installer `omarchy-install-service-spotify` (`omarchy-pkg-add spotify`), guarded like the menu: `omarchy-pkg-missing spotify` |
| Discord, Zoom | Omarchy's preinstalled web apps |
| `claude`/`codex`/`opencode`… | Omarchy's mise launchers |
| Apple Music, iCloud, Claude (old web apps), Loom, Raindrop.io | Dropped for now (re-add a line to `WEBAPPS` in `install.sh`) |
| Todoist | **Native app** since 2026-10-17 (was a web app from 2026-10-10): AUR `todoist-appimage`, which repackages Todoist's official Linux AppImage (downloaded from `electron-dl.todoist.net`, pinned SHA-256) and depends on AUR `appimagelauncher` (3.0.0 beta, built from source; registers a system-wide AppImage handler via `binfmt_misc`; its `appimagelauncherd` helper stays disabled). Todoist's own launcher passes `--no-sandbox`; tested without it: Chromium's user-namespace sandbox works (renderer in its own user/PID namespaces, `Seccomp: 2`), so `install.sh` writes a local `~/.local/share/applications/todoist.desktop` from the package's, without the flag and with `--password-store=gnome-libsecret`: Electron picks the keyring from `XDG_CURRENT_DESKTOP` and doesn't know Hyprland, so it stored its token unencrypted (*"Safe Storage Encryption is not available"*); with the flag it's in gnome-keyring (Omarchy's `chrome-flags.conf` sets the same for Chrome; its keyring item "Chromium Safe Storage", application `Todoist`). Left as is: Todoist's cookies (incl. its session cookie) are stored **unencrypted** in `~/.config/Todoist/Cookies` (mode 600) with or without the flag; Electron encrypts cookies only when the app enables it (Chrome's and Slack's are encrypted). `install.sh` removes the old web app. Building `appimagelauncher` installs build-only packages that yay leaves behind as unused dependencies (`appimagelauncher-debug`, `argagg`, `boost`, `doxygen`, `qt5-tools`; removed by hand here 2026-10-17); `install.sh` drops them right after a fresh Todoist install, only those `pacman -Qdtq` lists as unused. Others it pulled in stay: `appimagelauncher`'s runtime dependencies (Qt 5, `patchelf`, `nlohmann-json`, 32-bit glibc/gcc libs, `squashfuse`), `cmake` and `gvim` (now provides `vim`; optional for `fzf`). Closing its window quits it unless **Stay Running in Tray** (tray menu; `stay_in_tray` in `~/.config/Todoist/settings.json`) is on, which Todoist defaults to only on Windows (checked in its app code); `install.sh` sets that key with `jq` (Omarchy only: on the Mac, closing the window doesn't quit it). The `td` CLI stays a mise tool |

`WEBAPPS` entries are `"Name|URL|icon URL"`: without an icon URL, `omarchy-webapp-install` fetches the site's own icon (the old PNG icon files weren't kept). `install.sh` recreates a launcher whose URL changed. Making Slack open on a fixed workspace automatically is a **Desktop section** item.

## Bazecor (Dygma keyboard)

Decided 2026-10-10: on Omarchy and the Mac Studio (not the work laptop).
`install.sh` runs `omarchy-pkg-aur-add bazecor` (AUR `bazecor` 1.10.0:
Dygma's official AppImage, plus Dygma's udev rule `10-dygma.rules` for USB
access without root). Keyboard layouts are stored on the keyboard itself.

## Voxtype (dictation): Omarchy's installer

Was installed by hand from Omarchy's menu (Install > AI > Dictation), so
`install.sh` runs `omarchy-voxtype-install` when `voxtype-bin` is missing
(decided 2026-10-10). It asks to confirm, installs `wtype` + `voxtype-bin`,
copies Omarchy's `default/voxtype/config.toml`, downloads a model, sets up
its user service; Omarchy's Hyprland bindings give F9 (hold, push-to-talk)
and Super+Ctrl+X (toggle). Local transcription; the Macs get the same
([macos.md](macos.md)).

## Syncthing: Obsidian vaults across machines

Decided 2026-10-10. The vaults (mostly work, some personal) leave iCloud.
Considered: Dropbox (Omarchy has `omarchy-install-service-dropbox`, but the
free plan allows 3 devices and 2 GB base storage, and work notes would sit in
a personal cloud account), Obsidian Sync (paid, best on iPhone), Google
Drive (no official Linux client). **Chosen: Syncthing**: free, open source,
no device or storage limits, syncs directly between the user's machines
(no third-party copy; works anywhere over Tailscale; the always-on Omarchy
desktop acts as the hub). The work vault goes to all three machines, the
personal vault only to Omarchy and the Mac Studio. Caveat: iPhone clients
are limited (occasional reading only; revisit if needed).

`install.sh` on Omarchy: `omarchy-pkg-add syncthing` (Arch `extra`; Omarchy
has no installer), the user service `syncthing.service`, and firewall rules
for sync (22000 tcp/udp) and LAN discovery (21027/udp, per Syncthing's own
ufw profile) open only to private LANs and `tailscale0` (comment
`dots-syncthing`); the web UI stays on `localhost:8384`. Pairing devices and
sharing folders is manual ([setup](../setup/omarchy.md)).

**Not for git repos / `~/Work`:** concurrent writes to `.git` create
`.sync-conflict` files and corrupt repos, platform-specific files
(`node_modules`, `.venv`, compiled gems) break across Linux and macOS, and
macOS's case-insensitive file names conflict with Linux. Git (push/pull)
syncs repos; Syncthing is for notes and documents.

**Set up 2026-10-12** (pairing by hand, done over SSH with `syncthing cli`):

- **Folders:** the same paths on every machine, `~/Obsidian/<vault>`
  (not `~/Documents`: on the Macs that's iCloud's). `obsidian-work`
  (`~/Obsidian/Work`, the former iCloud vault, renamed from "Cloud" by
  copying it out of iCloud) on all three; `obsidian-personal`
  (`~/Obsidian/Personal`, new) only on Omarchy and the Mac Studio.
- **Omarchy is the hub:** each Mac pairs only with Omarchy, which relays
  between them, so the work MacBook never connects to the Mac Studio and
  can't be offered the personal vault. Device addresses: the Tailscale IP
  plus `dynamic`.
- **Ignored:** `.obsidian/workspace*.json` (`.stignore` in each vault, not
  synced itself): Obsidian rewrites the window layout constantly, which
  would make conflict copies; settings and plugins still sync.
- **Versioning (staggered, 365 days) on Omarchy and the Mac Studio**, both
  vaults: Syncthing has no off-site copy, so a deleted or damaged note
  would spread everywhere. Versions land in each vault's `.stversions`
  (never synced): one per 30 s for an hour, hourly for a day, daily for 30
  days, then weekly. A version is kept only for changes made **on another
  device** (`man syncthing-versioning`), so one machine isn't enough: with
  both, every change has a version on a machine other than where it was
  made. Not on the work MacBook (redundant, and history on the company
  machine). Tested both ways (edits on the MacBook kept on Omarchy, edits
  on Omarchy kept on the Mac Studio).
- Verified: the 262 vault files hash the same on all three machines; the
  MacBook has only the work folder.
- **Scripted** (2026-10-12, `setup_syncthing_vaults` in `install.sh`, both
  systems): the vault folders with their `.stignore`, Personal and
  versioning by profile (personal only); on a Mac also the device name
  (its model, when still the default hostname) and Omarchy as the hub
  (`SYNCTHING_HUB_ID`, `SYNCTHING_HUB_ADDRESS` in `lib/dots.sh`; a device
  ID is safe to publish, it can't connect without approval), sharing its
  vaults with it. Left by hand: approving a new Mac on Omarchy (the
  deliberate step), the GUI password, opening the vaults in Obsidian.
  Existing folders, shares and hand-set names are left alone. Tested on
  throwaway Syncthing instances (Omarchy personal; MacBook work: no
  Personal, no versioning) and against the real three (no change).
- **GUI password:** set by hand on each machine (Settings > GUI), stored
  in 1Password. The GUI and API listen on 127.0.0.1 only; without a
  password any program under the user could change Syncthing. `syncthing
  cli` uses the API key, so the scripts don't need it.

## Still to review

- **Group A**: done. (`tmux-ssh` moves to the bin scripts
  section.)
- **Group D** (not installed): `yarn` (done: mise), `rclone` (not set up; see [rclone guide](../guides/rclone.md)), `awsvpnclient` (not via dots; see [desktop.md](desktop.md)), `espanso` (dropped; see [desktop.md](desktop.md)).
