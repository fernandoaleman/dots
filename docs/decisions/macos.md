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
differences are conditions on the profile. The Brewfile reads it from
`~/.config/dots/profile`, not `ENV["DOTS_PROFILE"]`: `brew` re-executes
itself with a cleaned environment (`env -i`, keeping only `HOMEBREW_*` and
a short list such as `HOME` and `PATH`; end of `/opt/homebrew/bin/brew`),
so the variable never reaches the Brewfile. Found on the first Mac run,
which installed the personal-only apps on the work laptop; without a
profile file the Brewfile now refuses to run instead of guessing. Mac App Store (`mas`)
apps need a signed-in Apple ID, so they're personal-profile only.

Third-party taps (Voxtype's): Homebrew 7 refuses to load their formulae and
casks until trusted (`Library/Homebrew/trust.rb`), so each such entry is
fully qualified with `trusted: true`, trusting that one item rather than
the whole tap (found on the first Mac run, 2026-10-10).

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
  `~/Work/.mise.toml` (adds a project's `bin/` to PATH); built in
  [mise on the Mac](#mise-omarchys-setup-plus-dots-tools).

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
| Docker | formulas `colima`, `docker`, `docker-compose`, `docker-buildx` | both | CLI only (the GUI is never used; the company's Docker license stays available): Colima runs Docker Engine in a VM; `docker compose`/`buildx` as CLI plugins (`cliPluginsExtraDirs` in `~/.docker/config.json`); lazydocker via Colima's Docker context. Settings: [Docker (Colima)](#docker-colima) |
| Claude Code, Codex, opencode | mise (global), not casks | both | like Omarchy: `mise use -g` with Omarchy's `claude`, `codex` (+ `gh`, `node`, `prek`) plus `opencode`; casks `claude-code`/`codex` dropped (two copies would fight over the command) |
| ChatGPT (with Codex) | cask `chatgpt` (native) | both | work-paid; mirrors Omarchy's native `openai-codex-desktop` |
| Claude desktop | cask `claude` (native) | both | work-paid (Omarchy has no Claude desktop app) |
| Gemini desktop | dropped | n/a | personal account; browser (or a Chrome web app) when needed |
| Discord | Chrome web app (manual) | personal only | personal account: Mac Studio only, as a web app like Omarchy's built-in one; not on the work laptop |
| AppCleaner | cask `appcleaner` | both | installed as a fallback only: remove cask apps with `brew uninstall --cask --zap <name>` (deletes the cask's listed leftovers); AppCleaner for anything installed outside dots |
| balenaEtcher | dropped | n/a | install when needed (cask `balenaetcher`) |
| LocalSend | cask `localsend` | personal only | mirrors Omarchy (base package); file sharing between personal Macs and Omarchy. Work laptop: Taildrop covers moving files |
| Logi Options+ | cask `logi-options+` | personal only | for the Logitech device at the Mac Studio desk |
| Loom | dropped | n/a | no longer used |
| MacDown, MarkText | dropped | n/a | both casks are disabled in Homebrew; Neovim's Markdown preview, Obsidian and QLMarkdown cover it |
| Obsidian | cask `obsidian` | both | already on Omarchy (base package). Vaults move from iCloud to Syncthing (below) |
| Syncthing | formula `syncthing` (started at login) | both | syncs the **Obsidian vaults only**: the work vault on all three machines, the personal vault only on Omarchy and the Mac Studio. See [packages.md](packages.md) |
| Plaud | cask `plaud` | both | records Slack huddles, Teams and Zoom calls without a meeting bot; Plaud Desktop is Mac/Windows only. Omarchy: see the [TODO](../TODO.md) |
| QLMarkdown | cask `qlmarkdown` | both | renders `.md` in Finder's Quick Look (Space); local only |
| Raindrop.io | dropped for now | n/a | decide with the bookmarks TODO (Raycast quicklinks are going away; the Omarchy catalog has a Raindrop Bookmarks picker) |
| Spotify + Reflex | casks `spotify`, `reflex-app` | personal only | personal account (Omarchy installs Spotify too); Reflex sends the Mac's media keys to Spotify instead of Apple Music |
| Todoist | cask `todoist-app` | both | needed on every machine (used mostly for work, with some personal tasks). Omarchy: a web app (`install.sh` `WEBAPPS`; no official Arch package). The 1Password item "Todoist (Personal)" (`TODOIST_API_TOKEN` for `td` and the agent skills) moves to the **work** 1Password account with the other work items, so `td` works on the work laptop too |
| Voxtype (dictation) | tap `peteonrails/voxtype`, cask `voxtype` | both | same dictation as Omarchy, fully local (*"No cloud, no subscription, no telemetry"*), so fine on the work laptop; macOS build with Metal. Set up by `install.sh`, see [Voxtype](#voxtype-dictation) |
| Wispr Flow | dropped | n/a | no Linux version (macOS, Windows, iOS, Android only); paid, and the free tier runs out quickly. Voxtype instead |
| KeyStats | dropped | n/a | keystroke/mouse statistics (third-party tap); needs Input Monitoring, i.e. sees every key |
| TablePlus | dropped | n/a | not used (cask `tableplus` exists) |
| Bazecor | cask `bazecor` | personal only | Dygma keyboard configurator (layouts live on the keyboard). Omarchy too: AUR `bazecor` via `omarchy-pkg-aur-add` ([packages.md](packages.md)) |
| TickTick | dropped | n/a | Todoist replaced it |
| Pixelmator Pro | dropped | n/a | App Store only; Pinta (Omarchy) for image editing |
| AdGuard Mini | dropped | n/a | Safari-only (App Store); Chrome is the browser (ad blocking via a Chrome Web Store extension, synced with the profile) |
| Amphetamine → KeepingYouAwake | cask `keepingyouawake` | both | Amphetamine is App Store only; KeepingYouAwake is a free menu-bar "stay awake" toggle (built-in fallback: `caffeinate -d`) |
| AWS Extend Switch Roles (Safari, App Store) | dropped | n/a | the Chrome extension (Chrome Web Store) is the one used; set up in each machine's Chrome profile (checklist) |
| Keynote, Numbers, Pages | dropped | n/a | App Store only (needs an Apple ID); LibreOffice instead |
| LibreOffice | cask `libreoffice` | both | same office suite as Omarchy (`libreoffice-fresh`); Office on the web for Microsoft formats when needed |
| SonicWall Mobile Connect | dropped | n/a | the AWS VPN replaced the SonicWall VPN. Archive its 1Password login in the 1Password cleanup |
| Alacritty | dropped | n/a | not installed on Omarchy after all (only Omarchy's config template); Homebrew disabled its cask on 2026-09-01 (`fails_gatekeeper_check`: not notarized), so the Mac would need a Gatekeeper override. foot on Omarchy, Ghostty on the Macs |
| MacX YouTube Downloader → yt-dlp | formula `yt-dlp` | personal only | the same command-line downloader Omarchy ships, instead of the closed-source GUI app |

### Brewfile section 3: extra command-line tools

Decided 2026-10-10, from the Mac Studio's remaining formulae:

- **Kept:** `just` (via **mise**, on every machine), `pandoc`, `mkcert`
  (Brewfile on the Macs; `pandoc-cli` and `mkcert` from Arch on Omarchy,
  which already has `nss` so Chrome trusts mkcert's certificates), `rsync`
  (Macs only: macOS's own `rsync` is limited; Omarchy has the real one).
- **Dropped:** `cloudflare-wrangler`, `bats-core`, `pipx` (`uv tool install`
  instead), `coreutils` (`install.sh` uses macOS-compatible commands),
  `curl` (macOS's is fine), `switchaudio-osx`, `zlib` (a dependency), `nss`
  (Firefox only on the Mac), `mas` (no App Store apps left).
- Already decided elsewhere: dropped on Omarchy (`act`, `bottom`, `htop`,
  `colordiff`, `markdownlint-cli2`, `ncdu`, `thefuck`, `gitmux`, `sesh`,
  `hunk`); the old setup (`chezmoi`, `age`, `zsh` + plugins,
  `reattach-to-user-namespace`); via mise (`yarn`, `todoist-cli`, `gh`,
  `opencode`); `tailscale` (the app), `aws-vpn-cli`.

## Bash: Omarchy's defaults plus a Mac add-on

Decided 2026-10-10. **Omarchy** (4.0.4, checked in `/usr/share/omarchy`):
`~/.bashrc` is its template (`default/bashrc`), which sources
`default/bash/env-bootstrap` (`OMARCHY_PATH`, mise shims, `~/.local/bin`) and
then `default/bash/rc` (`envs`, `shell`, `aliases`, `functions`, `init`,
`inputrc`); dots' include line ends it. `~/.bash_profile` comes from Arch's
bash package and only loads `~/.bashrc`.

On a Mac those defaults can't run as is: the paths are fixed to
`/usr/share/omarchy` (macOS doesn't allow writing to `/usr/share`); `EDITOR`
and `BROWSER` call `omarchy-launch-*`; `open()` is redefined as `xdg-open`;
bash-completion and fzf load from `/usr/share`; and they need bash 4+
(`local -A`, `[[ -v`), while macOS's `/bin/bash` is 3.2.

So, on macOS:

- **Omarchy's real defaults, pinned:** `install.sh` downloads Omarchy's
  release tarball at `OMARCHY_REF` (`lib/dots.sh`, `v4.0.4`, the desktop's
  version) and keeps `default/bash/` and `default/bashrc` in
  `~/.local/share/omarchy/` (same layout as `/usr/share/omarchy`; re-downloaded
  only when the pin changes). No copies in dots. Bump the pin deliberately.
- **`~/.bashrc`:** Omarchy's template, seeded once if missing, plus dots'
  usual include line. Its first line (`/usr/share/omarchy/…/env-bootstrap`)
  is skipped on the Mac because the file doesn't exist there.
- **`macos` stow package** (Mac only), the add-on in two parts:
  - `~/.bash_profile`: loads `~/.config/bash/macos-env`, then `~/.bashrc`.
    `macos-env` (exports only, inherited by child shells) does
    env-bootstrap's job: Homebrew's `brew shellenv`, `OMARCHY_PATH`, mise
    shims and `~/.local/bin`, plus `EDITOR=nvim` and `BROWSER=open`
    (Omarchy's `envs` keeps values already set, so nothing is patched).
  - `~/.config/bash/macos`, loaded first by `~/.bashrc.dots`: restores
    macOS's `open`, loads Homebrew's bash-completion and `fzf --bash`.
    dots' git completion for `g` also tries Homebrew's path.
- **Login shell:** Homebrew's bash 5 (`/opt/homebrew/bin/bash`, added to
  `/etc/shells`, `chsh`), so Ghostty, Terminal, tmux and SSH get the same
  shell as on Omarchy. Rejected: keeping zsh and starting bash only in
  Ghostty (SSH and other tools would land in zsh).

Known gap: Omarchy's `sff` uses GNU `find -printf`, which macOS's `find`
lacks (troubleshooting).

## Omarchy's config files on the Mac

Decided 2026-10-10. **Omarchy** copies its config templates
(`/usr/share/omarchy/config/`) into `~/.config` once at install; after that
they're the user's files, and dots adds include lines to git's `config` and
`tmux.conf`.

On macOS, `install.sh` does the same from the pinned download
(`OMARCHY_REF`; kept in `~/.local/share/omarchy/config/`): it copies
`OMARCHY_TEMPLATES` (`lib/dots.sh`: git, tmux, starship, lazygit, btop,
herdr, Ghostty, opencode) into `~/.config` **only when missing**, then
stows `git` and `tmux` and adds the same include lines, plus the tmux
plugins at the same pinned commits. Skipped: Omarchy's Linux desktop
templates (Hyprland, foot/kitty/alacritty, chromium flags, fcitx5, imv,
wireplumber, xournalpp, autostart).

- **Ghostty** (the Mac's terminal; Omarchy's is foot): Omarchy's config
  plus `~/.config/ghostty/config.dots` (`macos` package), loaded by an
  include line: `macos-option-as-alt = true` (tmux's Alt keys) and
  `font-size = 13` (Omarchy's 9 suits its desktop; 13 is also Ghostty's Mac
  default, so `ghostty +show-config` only lists it with
  `--changes-only=false`). Omarchy's Linux-only lines (`async-backend =
  epoll`, `gtk-toolbar-style`) pass `ghostty +validate-config` on macOS.
- **Ghostty theme: OneDark "dark"** (`~/.config/ghostty/themes/onedark-dark`,
  `theme = onedark-dark` in `config.dots`), copied from
  [onedark.nvim](https://github.com/navarasu/onedark.nvim) (MIT) at a
  recorded commit: `palette.lua`'s `dark` colors mapped to the 16 terminal
  colors as its `terminal.lua` does, plus its `Normal`, `Cursor` (reverse)
  and `Visual` highlights for background, cursor and selection. Omarchy's
  theme include (`~/.local/state/omarchy/current/theme/ghostty.conf`) is
  optional and absent on the Mac. Making it an Omarchy theme too stays in
  TODO.
- **tmux `prefix ?`** calls `omarchy-menu-tmux-keybindings`; its `--print`
  mode needs only tmux and awk, so that one script is copied from the same
  download into `~/.local/share/omarchy/bin` (on PATH via `macos-env`).
- **btop** uses `color_theme = "current"`, Omarchy's theme link, which the
  Mac doesn't have, so btop shows its default colors. Left as is.

Seeded files are never overwritten on re-runs: a newer `OMARCHY_REF` only
changes `~/.local/share/omarchy`; compare and merge by hand (as on Omarchy,
where updates don't rewrite your `~/.config` either).

## Neovim: Omarchy's LazyVim, rebuilt from its sources

Decided 2026-10-11. **Omarchy** ships `~/.config/nvim` in the
`omarchy-nvim` package (`omacom/omarchy-pkgs`, `pkgbuilds/omarchy-nvim`):
its PKGBUILD downloads [LazyVim's starter](https://github.com/LazyVim/starter),
copies Omarchy's `lua/`, `plugin/` and `lazyvim.json` over it, and runs
`Lazy! sync` to ship a plugin cache; new users get both through
`/etc/skel`, plus a `theme.lua` link to the current Omarchy theme. dots
then stows the `nvim` package's `.dots.lua` files and merges its LazyVim
Extras ([nvim.md](nvim.md)).

On macOS, `install.sh` builds the same config from the same sources at
pinned commits (`OMARCHY_NVIM_REF`, `LAZYVIM_STARTER_REF` in `lib/dots.sh`)
and copies it to `~/.config/nvim` **only when missing**. Rebuilt at
omarchy-nvim 2026.8.13-1's commit, it matched this Omarchy machine's
`/etc/skel/.config/nvim` file for file, except `lazy-lock.json` and
`theme.lua`, which the build generates. Then the same `nvim` package and
Extras as on Omarchy (`add_nvim_extras`, shared by both systems).

- **Pin: `b2e3469`**, not 2026.8.13-1's own commit (`f20649b`): stable
  points `monokai-pro.nvim` at a deleted repo, which fails a fresh install
  (*"Too many rounds of missing plugins"*); `b2e3469` is that commit plus
  only Omarchy's one-line fix. Bump deliberately, usually to what the
  Omarchy desktop's package was built from.
- **No plugin cache:** the package's cache holds Linux builds (blink.cmp's
  fuzzy library, Tree-sitter parsers, Mason's tools), so lazy.nvim
  downloads the plugins on the first `nvim` start (latest commits, as
  Omarchy's own update check does) and Mason installs Mac builds of its
  tools. Neovim and `tree-sitter` come from the `Brewfile`.
- **Theme:** no Omarchy themes on the Mac, so no `theme.lua` link (it
  would point nowhere). The `macos` package adds
  `lua/plugins/theme.dots.lua`: [onedark.nvim](https://github.com/navarasu/onedark.nvim)
  with `style = "dark"`, the palette Ghostty's `onedark-dark` theme is built
  from, in the same shape as Omarchy's theme specs. Omarchy's
  `omarchy-theme-hotreload.lua` stays, idle (it only acts on theme
  switches).
- **Clipboard: Neovim's own `pbcopy`/`pbpaste` outside SSH.** Omarchy's
  `remote_clipboard.lua` (inside tmux, SSH or herdr) copies as OSC 52 and
  pastes from Wayland (`wl-paste`), asking the terminal only when there's no
  Wayland. The Mac has none, so inside tmux every paste asked tmux, whose
  `get-clipboard buffer` (its default) answers only from tmux's own paste
  buffers: text copied in other apps failed with *"Waiting for OSC 52
  response"* and E353 (tested 2026-10-11). `lua/config/options.dots.lua`
  (`macos` package, loaded last by an include line in Omarchy's
  `options.lua`) clears `vim.g.clipboard` unless in an SSH session, so
  Neovim picks `pbcopy`/`pbpaste`, the Mac's counterpart of Omarchy's
  `wl-paste` path. Over SSH, Omarchy's OSC 52 behavior stays. Not chosen:
  tmux `get-clipboard request` plus Ghostty `clipboard-read = allow`, which
  would let any program in the terminal (remote hosts included) read the
  clipboard without asking.

## mise: Omarchy's setup plus dots' tools

Decided 2026-10-11. **Omarchy** sets mise up at install
(`install/user/mise.sh`, `install/user/mise-work.sh`): `mise settings set
upgrade.auto_prune false` (so `mise up` never deletes a version a running
tool executes from), `~/Work` and `~/Work/tries`, `~/Work/.mise.toml`
(`[env] _.path = "{{ cwd }}/bin"`, trusted: a project's `bin/` on PATH
anywhere under `~/Work`) and `mise use -g node@latest`. Coding tools get
first-run launchers in `~/.local/bin` from `omarchy-mise-install` (about 15:
claude, codex, gh, opencode, crush, gemini, copilot, playwright, pi, omp,
grok, cursor-agent, ghui, hunk, muse), each running `mise use -g <tool>`
the first time and then the tool. dots adds `conf.d/config-dots.toml` and
`mise install --yes` ([packages.md](packages.md)); `prek` comes from the
repo's `make setup`.

On macOS, `install.sh` does the same after the `Brewfile` (which installs
mise):

- the auto-prune setting, `~/Work/tries`, `~/Work/.mise.toml` (written only
  if missing) with `mise trust`, and global Node (only if not set yet);
- launchers from **Omarchy's own `omarchy-mise-install`**, copied from the
  pinned download into `~/.local/share/omarchy/bin`, for the Mac's coding
  tools only: **claude, codex, gh, opencode** (the work laptop doesn't need
  the rest; `omarchy-mise-install <tool>` adds one);
- the `mise` stow package and `mise install --yes`: every tool in
  `config-dots.toml` has a macOS build in mise's registry (tested: all 12
  install and resolve, including aws-cli from AWS's `.pkg` without sudo);
- `prek`: `make setup` in `~/Work/dots`, as on Omarchy.

Node and Go are also what Mason needs for npm- and go-built tools
(json-lsp, yaml-language-server, pyright, gopls...); with them, all 28 of
LazyVim's Mason tools install on the Mac.

Differences kept:

- **Updates:** Omarchy runs `mise up` on every `omarchy update`; the Mac
  has no equivalent, so run Omarchy's `mup` alias now and then (re-running
  `install.sh` installs missing tools, it doesn't upgrade). A Mac update
  command may come with `dots doctor`.
- **Ruby:** Arch's 3.4 on Omarchy; on the Mac, Homebrew's 4.0 (a
  dependency of `try`). Projects pin theirs in `.ruby-version`, which mise
  reads ([Ruby guide](../guides/ruby.md)).
- **Python:** Apple's 3.9 (`/usr/bin/python3`); uv brings its own Python
  for ansible and yamllint.

## Docker (Colima)

Decided 2026-10-11. **Omarchy** runs Docker Engine directly:
`/etc/docker/daemon.json` (package `omarchy-settings`) sets log rotation
(`json-file`, 10m × 5), `dns` and `bip`; dots moves the networks off
`172.17.0.0/16`, which the work VPN routes (bridge `172.31.0.1/16`,
compose pools `192.168.128.0/17`, container DNS on `172.31.0.1`, the
host's resolver listening on the bridge), and makes Docker sudoless
(opt-in) ([desktop.md](desktop.md)).

On macOS, **Colima** runs Docker Engine in a Linux VM (Apple's
Virtualization framework); the Homebrew `docker`, `docker-compose` and
`docker-buildx` formulas are the CLI. `install.sh`:

- **First setup only:** writes `~/.colima/default/colima.yaml`. Colima
  takes a config file as is (`yaml.Unmarshal` into an empty struct,
  `config/configmanager`; unset keys become zero, e.g. disk 0), and its
  template (`colima template`) hardcodes `vmType: qemu` and
  `mountType: sshfs` though the command's defaults on macOS 13+ are `vz`
  and `virtiofs` (`cmd/start.go`). So `install.sh` has Colima write its
  full default template (`colima template --editor true`, removed again if
  it didn't exist) and sets `cpu: 4`, `memory: 8` (`COLIMA_CPUS`,
  `COLIMA_MEMORY` in `lib/dots.sh`; this Mac has 12 cores, 36 GB),
  `vmType: vz`, `mountType: virtiofs` and the `docker:` section, which
  becomes the VM's `daemon.json`: Omarchy's log rotation, dots' `bip` and
  `default-address-pools`. Colima keeps its own additions (BuildKit, the
  containerd snapshotter, cgroupfs) when the section is set
  (`environment/container/docker/daemon.go`). Disk 100 GB and the
  writable `$HOME` mount stay Colima's defaults.
- **Not Omarchy's `dns`:** on Omarchy it points containers at the host's
  resolver on the bridge address; nothing listens there in the VM. Colima's
  VM resolves through macOS (tested: containers resolve and reach the
  internet). With the AWS VPN connected: to be checked.
- **Later runs** leave the file alone (Colima rewrites it, sorted, on each
  start; change settings with `colima stop` then `colima start --edit`)
  and only warn if dots' `bip` is missing.
- **`sshConfig: false`** (first setup, and switched on existing configs):
  with it on, Colima adds `Include ~/.colima/ssh_config` to `~/.ssh/config`
  on every start, creating the file if missing (`generateSSHConfig`,
  `app/app.go`), and `~/.ssh/config` is dots' stowed file, so the line would
  land in the repo. `colima ssh` still works (Colima keeps writing
  `~/.colima/ssh_config`); only `ssh colima` goes. Tested: after a Colima
  restart dots' file is unchanged.
- **Starts at login:** `brew services start colima` (Homebrew's caveat;
  its service runs `colima start -f`).
- **CLI plugins:** `docker compose` and `docker buildx` are Homebrew CLI
  plugins Docker finds through `cliPluginsExtraDirs` (Homebrew's caveat).
  Docker writes `~/.docker/config.json` itself (`currentContext`, logins),
  so `install.sh` merges that key with `jq` instead of stowing the file.
- **Docker context:** Colima activates its `colima` context
  (`currentContext` in `~/.docker/config.json`); lazydocker reads it
  (`determineDockerHost`, `pkg/commands/docker.go`: `DOCKER_HOST`, then
  the current context), so plain `lazydocker` works.
- **No sudo:** Colima's socket (`~/.colima/default/docker.sock`) belongs
  to the user; Omarchy's sudoless-Docker question doesn't apply.

Tested on the work MacBook: Docker reports 4 CPUs, 8 GiB, `json-file`;
the VM's `daemon.json` has the settings; the bridge is `172.31.0.0/16`, a
compose network got `192.168.128.0/24`; a container read and wrote a file
under `~/Work`; `docker compose` 5.6.0 and `buildx` 0.38.0 work.

## Tailscale: DNS off, and the `tailscale` command

Decided 2026-10-11. **Omarchy** keeps Tailscale DNS on (systemd-resolved
keeps per-link DNS apart) and has `tailscale` on PATH. On macOS, Tailscale
DNS must be off wherever the AWS VPN runs ([desktop.md](desktop.md#tailscale--the-aws-vpn-on-the-same-machine)).

- **`install.sh`** runs `tailscale set --accept-dns=false` through the
  app's own binary (`/Applications/Tailscale.app/Contents/MacOS/Tailscale`,
  with `TAILSCALE_BE_CLI=1`: the binary decides between app and CLI from
  environment variables, per Tailscale's CLI docs), only while Tailscale is
  signed in (`status --json`: `BackendState` `Running`) and `CorpDNS` is
  still on (`debug prefs`); otherwise it reports and moves on. The setting
  is kept in Tailscale's preferences.
- **`tailscale` command:** an alias in `~/.config/bash/macos` (the
  `macos` package), as Tailscale's docs suggest. Not the app's **Install
  CLI** (a launcher in `/usr/local/bin`), which needs an admin password in
  the app's GUI. Scripts use the full path.
- **Exit node:** not set by `install.sh`; "None" is the default, and
  forcing it would undo a deliberate choice.
- **Open at login** (2026-10-12): the app's "Launch at login" checkbox
  fails on the work Mac ([troubleshooting](../guides/troubleshooting.md#mac-tailscale-start-on-login-could-not-be-set-popup)),
  so `install.sh` adds the app to Login Items (Open at Login) with
  AppleScript (System Events `make login item`, as `voxtype setup
  app-bundle` does), when it isn't there. Tested on the work Mac: skipped
  when present; removed, then re-added; works over SSH without a prompt.

## macOS settings (`defaults`)

Decided 2026-10-11, reviewed one setting at a time from the old
`run_onchange_after_90-setup-osx-defaults.sh.tmpl` on macOS 27.0.1. Rule:
a setting goes into `install.sh` only after it was run on the MacBook and
checked (read back, plus macOS's own view of it: System Events, Finder's
preferences, the System Settings switch, or the effect on screen); a
setting equal to macOS's own default is skipped (a no-op). **Omarchy** has
no Dock and no system-wide text substitutions, and its file manager shows
extensions and its location; most kept settings mirror that.

`MACOS_DEFAULTS` in `install.sh` writes only what differs (value and type),
quits System Settings first (it can overwrite changes made while open),
restarts Dock/Finder when their settings changed, then runs
`activateSettings -u` so key repeat applies without logging out.

| Setting | Key(s) | Checked by |
|---|---|---|
| Dock auto-hide | `com.apple.dock autohide` | System Events `autohide of dock preferences` |
| Dock shows without delay (normal slide kept) | `autohide-delay 0` | on screen |
| No recent apps in the Dock | `show-recents false` | read-back |
| Spaces keep their order (Omarchy's fixed workspaces) | `mru-spaces false` | read-back |
| Save dialogs expanded (Omarchy's full file dialog) | `NSNavPanelExpandedStateForSaveMode`, `…2` | TextEdit's Save dialog on screen |
| Document apps open a blank file, no Open dialog | `NSShowAppCentricOpenPanelInsteadOfUntitledFile false` | TextEdit reported an "Untitled" document |
| All file extensions shown | `AppleShowAllExtensions` | Finder: `all name extensions showing` |
| Finder path bar | `com.apple.finder ShowPathbar` | on screen |
| No warning when changing an extension | `FXEnableExtensionChangeWarning false` | rename test |
| New Finder windows: home folder (Omarchy's file manager) | `NewWindowTarget PfHm`, `NewWindowTargetPath file://$HOME/` | Finder: `new window target`; Cmd+N |
| Key repeat as fast as the sliders go (Omarchy: 40/s, 250 ms, `default/hypr/input.lua`) | `KeyRepeat 2`, `InitialKeyRepeat 15`, **floats** | the System Settings sliders |
| Autocorrect, capitalization, double-space period, smart quotes **and dashes** off | `NSAutomatic…Enabled false` (dashes added: the same switch) | the System Settings switches |
| TextEdit plain text | `com.apple.TextEdit RichText false` (lands in its sandbox container) | Format menu shows "Make Rich Text" |
| No `.DS_Store` on network shares | `com.apple.desktopservices DSDontWriteNetworkStores` | read-back + [Apple's support article](https://support.apple.com/en-us/HT208209) (no share to test; takes effect at next login) |

**Skipped:** menu bar auto-hide (Omarchy's bar is always visible; the
notch row gains little), always-on scroll bars (macOS's automatic ones
match Omarchy's overlay bars), large sidebar icons (taste), Activity
Monitor every 2 s (btop is the monitor, already 2 s), screenshots as PNG
(macOS's default), F1-F12 as standard function keys (media keys stay the
default on the Macs). Dropped from the old script: `sudo -v` and its
keep-alive (no setting needs admin rights).

**Key repeat on macOS 27:** the Keyboard sliders show the repeat speed
*active now*, which macOS loads from preferences at login, and they write
**floats**. The old script's `-int` values were stored but never shown;
`activateSettings -u` (a private helper in
`SystemAdministration.framework`, skipped if missing) applies stored
values at once (tested: the sliders moved with each write). Also,
`defaults write` of a value numerically equal to the stored one is
ignored (int `2` over float `2` stays an integer), so `install.sh`
deletes a key before writing it.

## Doctor (`dots_doctor`, `make doctor`)

Omarchy has no drift check; `dots_doctor` (`lib/dots.sh`) is dots' own.
On Omarchy it runs after every `omarchy update` (post-update hook), from
`make doctor` and at the end of `install.sh`. Decided 2026-10-11 for the
Mac:

- **Same checks:** include lines, every stowed file still linked, nothing
  uncommitted in the packages or the repo. The link check used
  `realpath -m`, which macOS's `realpath` lacks (every file was reported
  unlinked); plain `realpath` gives the same result on both systems (a
  missing file or broken link doesn't resolve into the repo).
- **Docker networks:** besides Omarchy's `daemon.json`, it checks that
  Colima's `~/.colima/default/colima.yaml` still has `bip: 172.31.0.1/16`
  (`install.sh` writes that config only once).
- **When it runs:** at the end of the Mac `install.sh` and with `make
  doctor`; the Mac has no `omarchy update`, so no hook, and no
  notification (`--notify` is the hook's, Omarchy-only).
- Tested on the work Mac: clean apart from expected uncommitted test
  changes; a broken `~/.ssh/config` link and a Colima `bip` of 172.17 were
  each reported; Omarchy reports the same as before.

## Voxtype (dictation)

Omarchy (`omarchy-voxtype-install`) copies `default/voxtype/config.toml`
(Voxtype's hotkey **off**: Hyprland binds hold F9 and Super+Ctrl+X toggle),
downloads `base.en`, enables Vulkan if present and installs a systemd user
service. Decided 2026-10-11 for the Mac (Voxtype 0.7.5, checked with
`voxtype setup check`, `voxtype config` and its source), in `install.sh`:

- **Config:** Omarchy's file from the pinned download, seeded once to
  `~/Library/Application Support/voxtype/config.toml` (the Mac path, not
  `~/.config`), with `pause_media = false`: it pauses players with
  `playerctl`, Linux-only (a warning on every recording otherwise).
- **Model:** `voxtype setup --download` (`base.en`, as on Omarchy) when
  missing. No GPU step: the Mac build has Metal built in; `setup gpu` is
  Vulkan.
- **Running:** `voxtype setup app-bundle` (Voxtype's recommendation):
  `/Applications/Voxtype.app` in Login Items, the only way it gets
  Microphone, Input Monitoring and Accessibility (`setup launchd` doesn't
  get the microphone). It copies the binary and resets two permissions
  when that changes, so `install.sh` runs it only when the app is missing
  or its version differs from `voxtype --version` (after `brew upgrade`).
- **Recording: the menu bar**, hotkey off as in Omarchy's file. Tried hold
  F9: with media keys on it needs fn+F9, and Voxtype's macOS hotkey only
  *listens* (a listen-only event tap), so the key also reaches the app in
  front, which beeps on every key repeat. Right Command (no beep, but its
  shortcuts start empty recordings) and Hammerspoon (takes the key like
  Hyprland; [TODO](../TODO.md)) were the alternatives.
- Tested on the work Mac: transcription typed into TextEdit, from the menu
  bar and with fn+F9; Login Item present; re-runs change nothing.

## Syncthing

Omarchy: `install.sh` installs it, enables the systemd user service and
opens its ports in ufw to private LANs and Tailscale only
([packages.md](packages.md)). Decided 2026-10-11 for the Mac:

- **Service:** `brew services start syncthing` (Homebrew's LaunchAgent runs
  `syncthing --no-browser --no-restart`, kept alive, at login), skipped
  when already running. Web UI on `localhost:8384`, as on Omarchy.
- **No firewall step:** IT manages the work Mac's firewall (turned on
  2026-10-11, before IT does). Its rules are per app, not per network, so
  Omarchy's LAN/Tailscale-only rule can't be copied; Syncthing only talks
  to paired devices. If the firewall blocks it, sync still works by the Mac
  dialing out to Omarchy (the hub).
- Tested on the work Mac with the firewall on: the service starts and
  survives a re-run; 22000 is reachable over the LAN and Tailscale (the
  firewall lists `syncthing` as allowed); the GUI listens on 127.0.0.1
  only.

## HostName: from the model (`mac-studio`, `macbook-pro`)

Omarchy's hostname (`omarchy`) is set once at install. A Mac with no
HostName names itself after the network; with the AWS VPN connected that's
`ip-10-0-0-…us-east-2.compute.internal`, which `hostname` and apps that name
themselves (Syncthing's device name) pick up. Decided 2026-10-12:
`install.sh` sets HostName, only when unset, from the model name
(`system_profiler`: "Mac Studio" → `mac-studio`, "MacBook Pro" →
`macbook-pro`), matching `omarchy` and the hosts in dots' `~/.ssh/config`.
LocalHostName (the `.local` name) and ComputerName (Finder, AirDrop, likely
what IT's tools list) stay as macOS set them. Omarchy's prompt (starship)
doesn't show the host, so the prompt doesn't change. Replaces the Mac
Studio's manual step. Tested on the work Mac: set to `macbook-pro`, a
re-run skips it, and `hostname` stays `macbook-pro` with the AWS VPN
connected.
