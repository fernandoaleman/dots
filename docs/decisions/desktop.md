# Desktop

Started 2026-10-03 (in progress). Old source: `dotfiles/dot_config/hypr/`
(`hypridle.conf`, `monitors.conf`), plus desktop-related chezmoi scripts.

Omarchy 4 configures Hyprland in **Lua** (`~/.config/hypr/*.lua`, loaded by
`hyprland.lua` with `require("hypr.<name>")`) and handles idle/lock in its
own shell (`~/.config/omarchy/shell.json`); `hypridle` is no longer used
(not installed).

## Idle and lock: Omarchy's

| | Old `hypridle.conf` | Omarchy (kept) |
|---|---|---|
| Screensaver | 5 min | 2.5 min (`idle.screensaver: 150`) |
| Lock | 10 min | 5 min (`idle.lock: 300`) |
| Screen off | 15 min | ~5 s after the lock screen appears while idle (the lock's blank timer runs `omarchy-brightness-display off`) |

Lock before sleep and wake handling are built into Omarchy's lock service.
To change the timings: `idle.screensaver` / `idle.lock` in `shell.json`
(JSON, rewritten by `omarchy bar …`, so it would be a `jq` merge in
`install.sh`, not a stowed file).

## Monitors: `hypr` package, `monitors.dots.lua`

- `~/.config/hypr/monitors.lua` stays Omarchy's (catch-all rule: preferred
  mode, auto position, scale 1.6, `GDK_SCALE=2`). `install.sh` appends a
  line that loads `~/.config/hypr/monitors.dots.lua` **by path** (`dofile`,
  after checking the file exists; Lua `require` can't load a dotted name).
  `dots_include` writes the marker comment with `--` for Lua files.
- Monitors are matched by **description** (`desc:` + model + serial, per the
  Hyprland wiki's *Output selection*), not by port. The rules then only
  apply to these exact monitors; any other screen (e.g. a laptop panel)
  falls back to Omarchy's catch-all (*"a fallback rule to use when no other
  rules match"*). Port names like `HDMI-A-1` aren't unique across machines.
- Desk: three 32" curved LG UltraGear 1440p monitors, scale 1.6 (Omarchy's),
  positions in scaled pixels (2560 / 1.6 = 1600 per monitor):

  | Position | Port | Description serial | Mode | Workspaces |
  |---|---|---|---|---|
  | left | `DP-1` | `408BOPY0P208` | 2560x1440@180 | 1, 2, 3 (default 1) |
  | middle | `DP-2` | `407BOYQ0E522` | 2560x1440@180 | 4, 5, 6 (default 4) |
  | right | `HDMI-A-1` | `407BOPY0E520` | 2560x1440@144 (HDMI max) | 7, 8, 9 (default 7) |

  So `Super+N` always lands on the same physical monitor. (The old config
  only bound 1–3, so 4+ opened wherever focus was.) Hyprland can't know the
  physical order; it was identified by which workspace each bar showed.
- Existing workspaces stay where they were created; to re-home one without
  logging out, focus it and run
  `hyprctl dispatch 'hl.dsp.workspace.move({ monitor = "DP-1" })'` (Omarchy
  binds the same as Super+Shift+Alt+Arrow).
- Tested live (2026-10-03): no `hyprctl configerrors`; new workspaces open
  on their bound monitor from anywhere (Super+6 → middle, Super+8 → right);
  a workspace bound to a **disconnected** monitor simply opens on the
  connected one; the
  `monitors.local.lua` layer loads. (Under Lua config, `hyprctl dispatch`
  takes Lua: `hyprctl dispatch 'hl.dsp.focus({ workspace = "2" })'`.)
- Old `monitors.conf` (hyprlang, port names, scale 1, `GDK_SCALE=1`):
  replaced.

## Apps pinned to workspaces: `hyprland.dots.lua`

Loaded by one line appended to Omarchy's `~/.config/hypr/hyprland.lua`
(where Omarchy says *"Add any other personal Hyprland configuration
below"*), same `dofile`-if-exists pattern as monitors.

| App | Matched by (`initial_class`) | Workspace |
|---|---|---|
| Chrome | `google-chrome` | 1 (left) |
| tmux terminal (Super+Alt+Return) | `tmux` | 4 (middle) |
| Slack web app | `chrome-1000bulbs.slack.com__-Default` | 7 (right) |
| Teams web app | `chrome-teams.cloud.microsoft__-Default` | 7 (right, side by side with Slack) |
| Outlook web app | `chrome-outlook.office.com__-Default` | 8 (right) |
| Spotify | `Spotify` | 9 (right) |

- Window rules like `workspace` are **static**: evaluated once when the
  window opens, against `initialClass`/`initialTitle` (Hyprland wiki,
  *Window rules*). A tmux terminal opens as a plain `foot` window and only
  later gets its tmux title, so it can't be matched. Fix: **Super+Alt+Return
  is rebound** (`hl.unbind` first, as Omarchy's guide requires) to Omarchy's
  same command plus `--app-id=tmux` (`omarchy-launch-terminal --app-id=tmux
  bash -c "tmux attach || tmux new -s Work"`; `xdg-terminal-exec` supports
  `--app-id`, so it works for any terminal). Other terminals (Super+Return)
  open wherever you are.
- Launching a pinned app takes you to its workspace (not `silent`). A pin
  only applies when the window opens; it can then be moved anywhere
  (Super+Shift+N) and stays there.
- Exec-with-rules (`hl.dsp.exec_cmd(cmd, { workspace = … })`) was not used:
  it tracks the spawned PID, and Omarchy launches apps through `uwsm-app`,
  so the window's PID differs.
- Verified live: a test tmux terminal opened with class `tmux` on
  workspace 4; the Teams class matched the rule exactly. Windows already
  open were moved with `hl.dsp.window.move({ window = "address:…",
  workspace = "N", follow = false })`.

## Cloud drives (rclone): not set up

Google Drive (`~/GoogleDrive`) and iCloud Drive (`~/iCloudDrive`) mounts
from the old setup are **not installed** for now (2026-10-04). Findings, and
the full steps to bring them back with updated unit files, are in the
[rclone guide](../guides/rclone.md): the patched rclone fork is no longer
needed (iCloud SRP sign-in shipped in rclone v1.74.0; Arch has 1.75.1),
Omarchy only has `fusermount3`, and credentials would come from 1Password.

## AWS VPN client: not via dots

The old setup installed AUR `awsvpnclient` (5.3.1, which AWS now lists as
*"No longer supported"*; AWS's current Linux client is 6.2.0, Sept 2026)
and enabled `awsvpnclient.service`, with the connection profile kept
encrypted. **Dropped** (2026-10-04): on Omarchy the VPN client will come from
the user's own Omarchy plugin (installed separately, outside dots). Notes
from the research: plain OpenVPN / NetworkManager works with AWS Client VPN
only for certificate-based endpoints; SAML (browser sign-in) endpoints need
AWS's own client.

**Mac:** AWS VPN client via Homebrew, profile created by hand (one-time).

## Text snippets: Omarchy's XCompose; espanso dropped

The old setup installed **espanso** (AUR `espanso-wayland`); its config held
only espanso's stock examples plus `search_shortcut: SHIFT+ALT+SPACE`, so
nothing personal is lost. **Dropped** (2026-10-04).

Omarchy already has text snippets via the **Compose key** (CapsLock):
`~/.XCompose` includes Omarchy's emoji table
(`/usr/share/omarchy/default/xcompose`) and two identity snippets,
CapsLock, Space, `n` (name) and CapsLock, Space, `e` (email). Omarchy's
installer (`install/user/xcompose.sh`) writes them from
`$OMARCHY_USER_NAME` / `$OMARCHY_USER_EMAIL`, which can be blank, leaving
`""`. `install.sh` fills **only empty** entries from the git identity
(`git config --global user.name` / `user.email`), then runs
`omarchy-restart-xcompose`. `~/.XCompose` stays Omarchy's real file and
nothing personal is in the repo. Add more snippets to `~/.XCompose`
directly (it's machine-local).

A picker-style snippets/bookmarks plugin shared with Raycast on the Mac is
a post-dots idea ([TODO](../TODO.md)).

**Mac:** no Compose key; Raycast snippets (free tier) cover this.

## Apple keyboard F-keys (fnmode): Omarchy's

The old `run_once_after_13-enable-mac-keyboard-fnmode` script wrote
`options hid_apple fnmode=2` to `/etc/modprobe.d/hid_apple.conf` (the
top row on Apple-style keyboards sends F1-F12; Fn gives the media keys).
**Dropped** (2026-10-04): Omarchy's installer does the same in
`install/hardware/fix-fkeys.sh` (run by `hardware/all.sh` on every install,
*"Ensure that F-keys on Apple-like keyboards (such as Lofree Flow84) are
always F-keys"*), writing the same line when the file is missing. Verified
here: the file exists and `/sys/module/hid_apple/parameters/fnmode` is `2`.

**Mac:** System Settings, Keyboard, "Use F1, F2, etc. keys as standard
function keys" (Mac-phase, with the old `90-setup-osx-defaults`).

## Docker networks: moved off 172.17.0.0/16 (overrides Omarchy)

A work VPN routes `172.17.x.x`, which is Docker's default bridge network,
so with Docker running those hosts are unreachable. The old
`run_once_after_14-fix-docker-network-conflict` script replaced
`/etc/docker/daemon.json`. **Ported** (2026-10-04), adapted to Omarchy.

Docker's defaults (Docker docs, *Networking overview*): bridge
`172.17.0.1/16`; every other network (each `docker compose` project) comes
from `default-address-pools`, built-in `172.17`-`172.31` (/16 each), then
`192.168.0.0/16` (/20).

What Omarchy does (installed files), all tied to `172.17.0.1`:

1. `/etc/docker/daemon.json` (package `omarchy-settings`, a pacman
   *backup* file: local edits are kept on updates, Omarchy's later changes
   arrive as `.pacnew`): log rotation (`json-file`, 10m x 5),
   `"dns": ["172.17.0.1"]`, `"bip": "172.17.0.1/16"`.
2. `/etc/systemd/resolved.conf.d/20-docker-dns.conf`:
   `DNSStubListenerExtra=172.17.0.1` (the host's DNS answers containers on
   the bridge address).
3. `install/config/firewall.sh`: ufw allows container DNS from
   `172.16.0.0/12` and `192.168.0.0/16` to `172.17.0.1` port 53. (Its
   ufw-docker rules cover all of `10/8`, `172.16/12` and `192.168/16`, so
   they need no change.)

What `install.sh` does (values in `lib/dots.sh`), guarded:

- **daemon.json:** `jq` merge setting `bip` `172.31.0.1/16`, `dns`
  `["172.31.0.1"]` and `default-address-pools`
  `[{base: "192.168.128.0/17", size: 24}]` (compose networks out of
  `172.x` entirely); Omarchy's other keys kept. Restarts Docker if running.
- **resolved:** our own drop-in `30-dots-docker-dns.conf`, loaded after
  Omarchy's: `DNSStubListenerExtra=` (empty: *"all previous assignments are
  cleared"*, `man resolved.conf`) then `DNSStubListenerExtra=172.31.0.1`.
  Omarchy's file is left untouched.
- **ufw:** the same two DNS allow rules for `172.31.0.1`. Omarchy's
  `172.17.0.1` rules are left in place (harmless: nothing listens there).
- **Drift:** `dots_doctor` (`make doctor`, the post-update hook) reports
  when `bip` is no longer `172.31.0.1/16` or the drop-in is gone.

Verified live (2026-10-04): `docker0` is `172.31.0.1/16` with no
`172.17` route left; resolved listens on `172.31.0.1:53` only (Omarchy's
`172.17.0.1` listener cleared); a container resolved `github.com` via
`172.31.0.1`; a new network got `192.168.128.0/24`.

Not the old home network: `192.168.128.0/17` is only Docker's pool for
extra networks (the home LAN is `10.0.0.x`, outside every range here). The
old `log-opts` were Omarchy's already. Existing compose networks keep their
old subnets until recreated (`docker compose down && docker compose up`, or
`docker network prune`).

**Mac:** the old script was Linux-only; Docker Desktop keeps its bridge
inside its VM (to confirm in the Mac phase).

## Work Claude Code plugins: manual for now

The old `run_once_after_35-setup-claude-plugins` script added the work
plugin marketplace and installed its plugins with `claude plugin
marketplace add` / `claude plugin install`. Omarchy has nothing for this
(it only installs the `claude` CLI, through mise). **Kept manual**
(2026-10-04): a [setup step](../setup/omarchy.md), since the company may
change how the plugins are distributed ([TODO](../TODO.md)). The names
stay out of this public repo.

## Todoist CLI agent skills: `install.sh`

The old `run_once_after_36-setup-todoist` (Mac only; its sync scripts
copied the skill to other machines) installed the Todoist CLI's Claude Code
skill and only *checked* the login. Omarchy has nothing for it; `td` comes
from our mise tools. **Ported** (2026-10-04), per machine:

- `install.sh` runs `td skill install` for **`claude-code`**
  (`~/.claude/skills`: Claude Code, opencode) and **`universal`**
  (`~/.agents/skills`: Codex, opencode), guarded by `td skill list`
  (`td skill install` exits 0 even when the skill exists). Not `td`'s
  `codex` option: it writes `~/.codex/skills`, which OpenAI's Codex docs no
  longer list (Codex reads `$HOME/.agents/skills`). opencode reads
  `~/.config/opencode/skills`, `~/.claude/skills` and `~/.agents/skills`
  (opencode docs, *Skills*), so it sees both copies; whether it dedupes by
  name isn't documented (drop one if it misbehaves).
- `td` runs through `mise exec`, since mise's shims may not be on PATH
  during a fresh install.
- Login (`td auth login`, browser OAuth) is a
  [setup step](../setup/omarchy.md); `install.sh` only warns when logged
  out. The old `td-login` helper is reviewed with the bin scripts.

**Mac:** same commands and folders.

## Repo remote to SSH: `make ssh` (manual until secrets)

The old `run_once_after_90-set-chezmoi-remote-ssh` switched the dotfiles
repo's `origin` from HTTPS to SSH unconditionally. **Dropped**
(2026-10-04): dots has `make ssh` (HTTPS to SSH, no-op when already SSH),
run by hand once SSH keys exist ([setup](../setup/omarchy.md)); switching
before the keys are installed would break `install.sh`'s `git pull` on a
fresh machine. Omarchy has nothing for this. **Secrets section:** once
`install.sh` installs the SSH keys from 1Password, it runs `make ssh`
itself (when the key exists and GitHub accepts it), and the manual step
goes away.

**Mac:** same.

## Startup folders: Omarchy's

The old `run_once_before_01-create-dirs` created `~/.local/bin`,
`~/.local/state/zsh` and `~/.cache/zsh`. **Dropped** (2026-10-04): the zsh
folders went with zsh; `~/.local/bin` exists from the Omarchy install and
Omarchy's `default/bash/envs` puts it on `PATH`. Where bin scripts live is
decided in the bin scripts section. **Mac:** created when the Mac mirrors
Omarchy's bash setup.

## Login shell: Omarchy's bash

The old `run_once_after_10-set-shell` switched the login shell to zsh
(`chsh`, adding it to `/etc/shells`) and, on Linux, replaced `~/.bashrc`
with a stub that `exec`s zsh. **Dropped** (2026-10-04): the login shell is
Omarchy's `/usr/bin/bash` (bash 5.3) and the shell decision is bash;
replacing `.bashrc` would remove Omarchy's bash defaults and our
`.bashrc.dots` include. **Mac phase:** the same idea for Homebrew's bash 5
(macOS ships 3.2): add `/opt/homebrew/bin/bash` to `/etc/shells`, then
`chsh`.

## age key: deferred to the secrets section

The old `run_once_before_02-setup-age-key` fetched the age key from
1Password (`op document get`) so chezmoi could decrypt the repo's encrypted
files (SSH keys, AWS config, API tokens, VPN profile). dots keeps nothing
encrypted in the repo (secrets come from 1Password via `op`), so the key
isn't needed day to day. **Secrets section, first check:** is everything in
those encrypted files also in 1Password? If yes, age is dropped; if not,
decrypt once with the key and move the missing items into 1Password.

## SSH agent, keys and server: deferred to the secrets section

The old `run_once_after_12-setup-ssh-keychain` (Mac only, with secrets)
added the personal and work SSH keys to the macOS Keychain
(`ssh-add --apple-use-keychain`) so passphrases were remembered. Deferred
(2026-10-04) to the secrets section, where the keys are installed from
1Password as real files in `~/.ssh`. Wanted there:

- **No passing keys on the command line:** `ssh <host>` picks the right
  key by itself (`~/.ssh/config` `IdentityFile` per host) and passphrases
  are remembered (an agent with `AddKeysToAgent`).
- **SSH into this machine** from other computers.

What Omarchy has (checked 2026-10-04):

- **Agent:** none set up. Omarchy's defaults, configs and installers don't
  mention `ssh-agent`, `SSH_AUTH_SOCK` or `gcr-ssh-agent`; `SSH_AUTH_SOCK`
  is empty here. OpenSSH's `ssh-agent.socket` and GNOME's
  `gcr-ssh-agent.socket` (keyring-backed, like the Mac Keychain) are
  installed but disabled.
- **Server:** `omarchy-setup-security-sshd` installs openssh, enables
  `sshd`, `ufw limit 22/tcp` (rate limited) and authorizes a public key
  (`--key=`, GitHub or pasted). `sshd` is already enabled and running on
  this machine; Omarchy's hardening is in
  `/etc/ssh/sshd_config.d/10-omarchy-hardening.conf`. In the secrets
  section: run that script from `install.sh` (guarded), with the key from
  1Password.
- Also revisit with the bin scripts' `tmux-ssh`.

**Mac:** Keychain via `ssh-add --apple-use-keychain` (+ `UseKeychain` /
`AddKeysToAgent` in `~/.ssh/config`); Remote Login in System Settings for
the server side.

## nvim folder check: dropped

The old `run_before_04-check-nvim-dir` deleted `~/.config/nvim` (`rm -rf`)
when it wasn't a git repo, so chezmoi could clone the separate nvim repo.
**Dropped** (2026-10-04): on Omarchy that folder is Omarchy's LazyVim
config, which dots extends ([nvim.md](nvim.md)); the old repo is no longer
cloned (archiving it is in the [TODO](../TODO.md)). **Mac:** not needed
(the Mac seeds Omarchy's nvim config at a pinned tag).

## Removing Omarchy's configs: dropped

The old `run_once_before_05-remove-omarchy-conflicts` deleted Omarchy's
nvim, alacritty, tmux, git config, btop, lazygit, starship and ghostty
configs so chezmoi could replace them. **Dropped** (2026-10-04): the
opposite of [config pattern C](config-pattern.md) (Omarchy's files stay
real; ours load through one include line). `install.sh` backs up a real
file that blocks stow instead of deleting anything. **Mac:** not
applicable.

## Mac notes

Monitor arrangement and idle/lock are macOS System Settings; nothing here
carries over. App-to-workspace pinning would need a macOS window manager
(Mac-phase decision).
