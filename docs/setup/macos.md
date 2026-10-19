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
starts Colima (Docker), applies the macOS settings (Dock, Finder,
keyboard, text input), asks macOS to make Chrome the default browser
(click **Use Chrome**) and makes Homebrew's bash 5 your login shell. When
it's done, **quit and reopen Terminal** (and any Ghostty windows) to get
the new shell. Some casks ask for the password again or show macOS
permission prompts; that's expected
([macos.md](../decisions/macos.md#bootstrap-on-a-fresh-mac)). The first
`nvim` start downloads its plugins and tools; let it finish. To commit to
dots from the Mac, run `make setup` in `~/Work/dots` once (prek and its
git hooks) and copy the private-words list to `~/.config/dots/private-words`
(mode 600; see [omarchy.md](omarchy.md) step 8).

**Optional, to let an agent on Omarchy drive the setup over SSH:** System
Settings > General > Sharing > **Remote Login** on (Allow access: only your
user), then let Omarchy's personal key in. GitHub no longer publishes it
(no SSH keys there), so copy the public key from Omarchy (`op read
"op://Private/id_ed25519/public key"` or `~/.ssh/id_ed25519.pub`) and add it
on the Mac (public, not a secret):

```sh
mkdir -p ~/.ssh && chmod 700 ~/.ssh
[ -s ~/.ssh/authorized_keys ] && [ -n "$(tail -c1 ~/.ssh/authorized_keys)" ] && echo >> ~/.ssh/authorized_keys
echo 'ssh-ed25519 AAAA… (the public key)' >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
ipconfig getifaddr "$(route -n get default | awk '/interface:/{print $2}')"
```

On the work laptop, consider turning Remote Login off again when done (IT
can see it's on).

## 1. 1Password

Sign in to 1Password (**work laptop: only the company account**; personal
Macs: your personal account, and the company account too) and turn on
**Settings > Developer > Integrate with 1Password CLI** (dots reads secrets
with `op`). Re-run `install.sh`: it installs the SSH keys, AWS files and
tokens, approving the 1Password prompt when asked.
On the work laptop, move any login used for work too (e.g. GitHub, a
personal account used for both) from the personal vault to the company
account first: it only sees the company account.

**GitHub:** run `gh-role-login` **in a local terminal** (not over SSH, so
`gh` keeps the token in the Keychain). The work laptop signs in with the
company-only token from 1Password (tag `dots/gh`; create it first:
[bin.md](../decisions/bin.md#gh-role-login-bin-package-new-2026-10-11)), a
personal Mac with the browser. No SSH key for GitHub on any machine.

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
install icon in the address bar). Chrome may only offer them as **PWAs**
(progressive web apps): that's the same thing. Chrome adds them to
`~/Applications/Chrome Apps.localized/` as "Microsoft Teams (PWA)" and
"Outlook (PWA)", which Spotlight finds. Allow notifications when asked.

**Personal Macs only** (never the work laptop): also `https://discord.com/app`
as a web app, like Omarchy's built-in Discord web app.

## 4. Tailscale: sign in

`install.sh` installs the Tailscale app and adds it to Login Items (Open at
Login; the app's own checkbox may fail with "Start on Login could not be
set",
[troubleshooting](../guides/troubleshooting.md#mac-tailscale-start-on-login-could-not-be-set-popup)).
Open it and sign in.

Then **re-run `install.sh`** (if Tailscale wasn't signed in on its first
run): it turns **Use Tailscale DNS off** (`tailscale set
--accept-dns=false`). Required on every Mac that also runs the AWS VPN: on
macOS, Tailscale's DNS replaces the system's resolvers, pushing out the
VPN's, and then *nothing* resolves (websites, Claude's API, AWS)
([desktop.md](../decisions/desktop.md#tailscale--the-aws-vpn-on-the-same-machine)).
Leave **Exit Node: None** (the default).

Consequence: Tailscale short names (`omarchy`) don't resolve on the Mac;
dots' `~/.ssh/config` uses Tailscale addresses instead.

## 5. AWS VPN

`install.sh` installs AWS's client; import the work profile by hand: the
1Password Document tagged `dots/vpn` (work account). Connect it **after**
Tailscale is up. Sign in with the **AWS SSO** login's username, password
and code (work account), not AWS Role Switch's. (On Omarchy the user's own
plugin does this instead.)

## 6. Moonlight (remote desktop into Omarchy)

1. Open Moonlight (installed by `install.sh`).
2. Add the Omarchy desktop by its **Tailscale address `100.121.13.36`**
   (works at home and away). Moonlight shows a **PIN**.
3. On Omarchy, open **Sunshine Admin** (`https://localhost:47990`; login in
   1Password), **PIN** tab: enter it and name the Mac.
4. `install.sh` already sets Moonlight's **Capture system keyboard
   shortcuts: in fullscreen** (gear icon; if it shows off, Moonlight was
   running during install: quit it and re-run). Stream full screen
   (**Ctrl+Option+Shift+X**) so Cmd reaches Omarchy as Super, Cmd+Space
   included (Spotlight's hotkey on the Mac). **Ctrl+Option+Shift+Q** ends
   the stream, **Ctrl+Option+Shift+Z** releases the keyboard.

See [desktop.md](../decisions/desktop.md#remote-desktop-sunshine--moonlight-omarchys-via-installsh).

## 7. Terminal: use Ghostty

`install.sh` configures Ghostty to send **Option as Alt**, so Omarchy's
tmux Alt keys work the same (Alt+Enter splits, Alt+1..9 windows,
Alt+arrows) ([tmux.md](../decisions/tmux.md)). Use Ghostty, not Terminal:
Terminal would need Settings > Profiles > Keyboard > **Use Option as Meta
key** by hand.

## 8. Small settings

- **Work Mac: firewall on** (System Settings > Network > Firewall); IT
  manages it, so leave its options to IT.

## 9. Voxtype permissions

`install.sh` sets up Voxtype.app (dictation). If macOS asks whether the
terminal may control **System Events**, allow it (adds the Login Item).
Then in System Settings > Privacy & Security turn on **Voxtype** under
**Accessibility** and **Input Monitoring** (add it with **+** from
`/Applications` if missing), allow the **Microphone** prompt on first use,
and restart it: `pkill -f voxtype-bin; open /Applications/Voxtype.app`.
Record from its menu bar icon. Redo the two toggles after a Voxtype
upgrade.

## 10. Syncthing (Obsidian vaults)

`install.sh` starts Syncthing, names the device after the Mac's model, adds
Omarchy (the hub) and shares `~/Obsidian/Work` with it (personal Macs also
`~/Obsidian/Personal`, with versioning;
[packages.md](../decisions/packages.md#syncthing-obsidian-vaults-across-machines)),
plus `~/.claude/projects` and `~/.agents` for coding agents
([agents.md](../decisions/agents.md#sync-between-machines-syncthing)).
By hand:

1. **On Omarchy**, approve the new Mac and its folders
   ([omarchy.md](omarchy.md) step 7); the vaults then sync down. Before
   approving `claude-projects`, migrate the Mac's Claude Code sessions
   (`claude-migrate-projects --apply`, then `cp ~/.claude.json
   ~/.claude/`; see [TODO.md](../TODO.md)).
2. In the Mac's web UI (`http://localhost:8384`), Settings > GUI: set a
   username and password (1Password; the work Mac's in the work account).
3. In Obsidian: Open folder as vault > `~/Obsidian/Work` (and `Personal`).
