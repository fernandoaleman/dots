# Fresh Mac setup

Manual steps for a Mac, collected as they're decided. The Mac phase
(dots 1.x) turns what it can into `install.sh`; the rest is this checklist.
Decisions behind each step are linked.

## 0. Run install.sh

On the fresh Mac (signed in to a local account; on the work laptop, no
personal Apple ID), open Terminal and run the same one-liner as on Omarchy:

```sh
bash <(curl -fsSL https://raw.githubusercontent.com/fernandoaleman/dots/master/install.sh)
```

It asks for your password, installs Homebrew (and Apple's Command Line
Tools), bash 5, git, gum, stow and jq, clones dots into `~/Work/dots`, asks
**personal or work**, sets up bash (Omarchy's defaults plus the Mac
add-on) and Omarchy's Neovim, installs the `Brewfile` and the mise tools,
starts Colima (Docker), asks macOS to make Chrome the default browser
(click **Use Chrome**) and makes Homebrew's bash 5 your login shell. When
it's done, **quit and reopen Terminal** (and any Ghostty windows) to get the new shell. Some
casks ask for the password again or show macOS permission prompts; that's
expected ([macos.md](../decisions/macos.md#bootstrap-on-a-fresh-mac)). The
first `nvim` start downloads its plugins and tools; let it finish. To
commit to dots from the Mac, run `make setup` in `~/Work/dots` once (prek
and its git hooks).

**Optional, to let an agent on Omarchy drive the setup over SSH:** System
Settings > General > Sharing > **Remote Login** on (Allow access: only your
user), then let Omarchy's key in (it's the key GitHub publishes for the
account):

```sh
mkdir -p ~/.ssh && chmod 700 ~/.ssh
curl -fsSL https://github.com/fernandoaleman.keys >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
ipconfig getifaddr "$(route -n get default | awk '/interface:/{print $2}')"
```

On the work laptop, consider turning Remote Login off again when done (IT
can see it's on).

## 1. 1Password

Install 1Password, sign in, and turn on **Settings > Developer > Integrate
with 1Password CLI** (dots reads secrets with `op`).

## 2. Chrome profile

- **Work MacBook:** sign Chrome in with a Google account for your **work
  email** (create one with "use my current email" if work isn't on Google
  Workspace). Never your personal Google account there: it would sync your
  personal history, bookmarks and passwords onto a machine IT manages.
- **Personal Macs:** your personal Google account, as on Omarchy.
- In that Chrome profile, install the **AWS Extend Switch Roles** extension
  (Chrome Web Store) and paste its role configuration (the daily console
  role switching; it syncs with the profile only if Chrome sync covers
  extension settings, so keep a copy of the configuration).

## 3. Web apps (Teams, Outlook; Discord on personal Macs)

In Chrome (the profile for this machine), for each of
`https://teams.cloud.microsoft` and `https://outlook.office.com`: open it,
sign in, then **⋮ > Cast, save and share > Install page as app** (or the
install icon in the address bar). Chrome adds them to
`~/Applications/Chrome Apps/`, so Spotlight finds "Teams" and "Outlook".
Allow notifications when asked.

**Personal Macs only** (never the work laptop): also `https://discord.com/app`
as a web app, like Omarchy's built-in Discord web app.

## 4. Tailscale: Use Tailscale DNS **off**

Install the Tailscale app, sign in, set it to **launch at login**. Then, in
its menu (or Settings):

- **Use Tailscale DNS: off** (`tailscale set --accept-dns=false`). Required
  on every Mac that also runs the AWS VPN: on macOS, Tailscale's DNS
  replaces the system's resolvers, pushing out the VPN's, and then *nothing*
  resolves (websites, Claude's API, AWS)
  ([desktop.md](../decisions/desktop.md#tailscale--the-aws-vpn-on-the-same-machine)).
- **Exit Node: None.**

Consequence: Tailscale short names (`omarchy`) don't resolve on the Mac;
dots' `~/.ssh/config` uses Tailscale addresses instead.

## 5. AWS VPN

Install AWS's client (`brew install --cask aws-vpn-client`) and import the
work profile by hand: the 1Password Document `1000bulbs.ovpn` (work vault,
tag `dots/vpn`). Connect it **after** Tailscale is up. (On Omarchy the
user's own plugin does this instead.)

## 6. Moonlight (remote desktop into Omarchy)

1. Install Moonlight (`brew install --cask moonlight`).
2. Add the Omarchy desktop by its **Tailscale address `100.121.13.36`**
   (works at home and away). Moonlight shows a **PIN**.
3. On Omarchy, open **Sunshine Admin** (`https://localhost:47990`; login in
   1Password), **PIN** tab: enter it and name the Mac.
4. Moonlight's **gear icon** (top right of its main window; not the macOS
   menu): tick **Capture system keyboard shortcuts**, choose **in
   fullscreen**. Stream full screen (**Ctrl+Option+Shift+X**) so Cmd reaches
   Omarchy as Super, Cmd+Space included (Spotlight's hotkey on the Mac).
   **Ctrl+Option+Shift+Q** ends the stream, **Ctrl+Option+Shift+Z** releases
   the keyboard.

See [desktop.md](../decisions/desktop.md#remote-desktop-sunshine--moonlight-omarchys-via-installsh).

## 7. Terminal: use Ghostty

`install.sh` configures Ghostty to send **Option as Alt**, so Omarchy's
tmux Alt keys work the same (Alt+Enter splits, Alt+1..9 windows,
Alt+arrows) ([tmux.md](../decisions/tmux.md)). Use Ghostty, not Terminal:
Terminal would need Settings > Profiles > Keyboard > **Use Option as Meta
key** by hand.

## 8. Small settings

- **Function keys:** System Settings > Keyboard > "Use F1, F2, etc. keys as
  standard function keys" (Omarchy's `fnmode=2` equivalent;
  [desktop.md](../decisions/desktop.md)).
- **`~/.hushlogin`** (silences "Last login" in new shells;
  [dotfiles.md](../decisions/dotfiles.md)).
- **Mac Studio:** `sudo scutil --set HostName Fernandos-Mac-Studio`. With no
  HostName set, macOS names itself after a reverse DNS lookup, which shows
  up as `ip-10-0-0-30` while the AWS VPN is connected.

## 9. Syncthing (Obsidian vaults)

`install.sh` installs and starts Syncthing. Once per machine, in its web UI
(`http://localhost:8384`):

1. Set a GUI username and password (Settings > GUI; store it in 1Password).
2. **Add the other devices** by their Device ID (Actions > Show ID on each);
   Tailscale lets them find each other anywhere.
3. **Share folders:** the **work** vault with all three machines; the
   **personal** vault only between Omarchy and the Mac Studio (never the
   work laptop). Never share `~/Work` or git repos.
