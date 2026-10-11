# Fresh Mac setup

Manual steps for a Mac, collected as they're decided. The Mac phase
(dots 1.x) will turn what it can into a Mac `install.sh`; until then this
is the checklist. Decisions behind each step are linked.

## 1. 1Password

Install 1Password, sign in, and turn on **Settings > Developer > Integrate
with 1Password CLI** (dots reads secrets with `op`).

## 2. Chrome profile

- **Work MacBook:** sign Chrome in with a Google account for your **work
  email** (create one with "use my current email" if work isn't on Google
  Workspace). Never your personal Google account there: it would sync your
  personal history, bookmarks and passwords onto a machine IT manages.
- **Personal Macs:** your personal Google account, as on Omarchy.

## 3. Teams and Outlook web apps

In Chrome (the profile for this machine), for each of
`https://teams.cloud.microsoft` and `https://outlook.office.com`: open it,
sign in, then **⋮ > Cast, save and share > Install page as app** (or the
install icon in the address bar). Chrome adds them to
`~/Applications/Chrome Apps/`, so Spotlight finds "Teams" and "Outlook".
Allow notifications when asked.

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

## 7. Terminal: Option as Alt

Configure the Mac terminal (likely Ghostty) to send **Option as Alt/Meta**,
so Omarchy's tmux Alt keys work the same (Alt+Enter splits, Alt+1..9
windows, Alt+arrows) ([tmux.md](../decisions/tmux.md)).

## 8. Small settings

- **Function keys:** System Settings > Keyboard > "Use F1, F2, etc. keys as
  standard function keys" (Omarchy's `fnmode=2` equivalent;
  [desktop.md](../decisions/desktop.md)).
- **`~/.hushlogin`** (silences "Last login" in new shells;
  [dotfiles.md](../decisions/dotfiles.md)).
- **Mac Studio:** `sudo scutil --set HostName Fernandos-Mac-Studio`. With no
  HostName set, macOS names itself after a reverse DNS lookup, which shows
  up as `ip-10-0-0-30` while the AWS VPN is connected.
