# Fresh Mac setup

Manual steps for a Mac, collected as they're decided. The Mac phase
(dots 1.x) will turn what it can into a Mac `install.sh`; until then this
is the checklist. Decisions behind each step are linked.

## 1. 1Password

Install 1Password, sign in, and turn on **Settings > Developer > Integrate
with 1Password CLI** (dots reads secrets with `op`).

## 2. Tailscale: Use Tailscale DNS **off**

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

## 3. AWS VPN

Install AWS's client (`brew install --cask aws-vpn-client`) and import the
work profile by hand: the 1Password Document `1000bulbs.ovpn` (work vault,
tag `dots/vpn`). Connect it **after** Tailscale is up. (On Omarchy the
user's own plugin does this instead.)

## 4. Moonlight (remote desktop into Omarchy)

1. Install Moonlight (`brew install --cask moonlight`).
2. Add the Omarchy desktop by its **Tailscale address `100.121.13.36`**
   (works at home and away). Moonlight shows a **PIN**.
3. On Omarchy, open **Sunshine Admin** (`https://localhost:47990`; login in
   1Password), **PIN** tab: enter it and name the Mac.
4. Moonlight's **gear icon** (top right of its main window; not the macOS
   menu): tick **Capture system keyboard shortcuts**, choose **in
   fullscreen**. Stream full screen (**Ctrl+Option+Shift+X**) so Cmd reaches
   Omarchy as Super, Cmd+Space included (Raycast's hotkey on the Mac).
   **Ctrl+Option+Shift+Q** ends the stream, **Ctrl+Option+Shift+Z** releases
   the keyboard.

See [desktop.md](../decisions/desktop.md#remote-desktop-sunshine--moonlight-omarchys-via-installsh).

## 5. Terminal: Option as Alt

Configure the Mac terminal (likely Ghostty) to send **Option as Alt/Meta**,
so Omarchy's tmux Alt keys work the same (Alt+Enter splits, Alt+1..9
windows, Alt+arrows) ([tmux.md](../decisions/tmux.md)).

## 6. Small settings

- **Function keys:** System Settings > Keyboard > "Use F1, F2, etc. keys as
  standard function keys" (Omarchy's `fnmode=2` equivalent;
  [desktop.md](../decisions/desktop.md)).
- **`~/.hushlogin`** (silences "Last login" in new shells;
  [dotfiles.md](../decisions/dotfiles.md)).
- **Mac Studio:** `sudo scutil --set HostName Fernandos-Mac-Studio`. With no
  HostName set, macOS names itself after a reverse DNS lookup, which shows
  up as `ip-10-0-0-30` while the AWS VPN is connected.
