# Fresh Omarchy setup

Steps for a brand-new Omarchy install, in order. Only steps that **can't**
be scripted are listed here; anything Omarchy has a script for runs
automatically in `install.sh` instead.

## 1. Run the dots bootstrap

```sh
bash <(curl -fsSL https://raw.githubusercontent.com/fernandoaleman/dots/master/install.sh)
```

It asks for your git name and email if the Omarchy installer didn't set
them.

**Secrets need 1Password signed in.** `install.sh` installs the 1Password
app, but signing in can't be scripted. On a fresh machine the secrets
step is skipped with a warning; then:

1. Open **1Password** and sign in.
2. **Settings > Developer > Integrate with 1Password CLI**: on.
3. Re-run `install.sh` and approve the 1Password prompt.

It then installs the SSH keys, `~/.aws` files, the API token env file (Todoist
included, so `td` needs no login) and
incoming SSH (sshd), and switches this repo's remote to SSH. Then open a new terminal. See the [README](../../README.md#usage) for
what `install.sh` does.

## 2. Slack (and Teams/Outlook) web apps

`install.sh` creates the Slack, Teams and Outlook web apps. Once:

- Open **Slack** and sign in; allow notifications when Slack asks (or via
  the site's permission icon in the app window).
- Keep the Slack window **open, parked on a workspace**: a closed web app
  sends no notifications.

## 3. Log in to GitHub

`gh` is preinstalled by Omarchy (through mise).

```sh
gh auth login      # GitHub.com → HTTPS → web browser
```

No `gh auth setup-git` needed: dots' git config already uses gh for GitHub
over HTTPS.

Use HTTPS until SSH keys are installed, then run `make ssh` in
`~/Work/dots` to switch the remote.

## 4. Work Claude Code plugins

Not automated (for now): after `gh auth login` (the marketplace repo is
private), in Claude Code run `/plugin`, add the work plugin marketplace
(its GitHub `org/repo`) and install the work plugins from it. The names
are kept out of this public repo; the old chezmoi script
(`run_once_after_35-setup-claude-plugins`) has the list.

Same on the Mac.

## 5. Remote desktop (Sunshine) for your Mac

`install.sh` installs and starts Sunshine. Then, once:

1. Open **Sunshine Admin** at `https://localhost:47990` in the browser. It
   uses a self-signed certificate: accept the warning for that site only.
   Create the admin username and password (store them in 1Password).
2. On the Mac, open **Moonlight** and add this machine (it appears on the
   LAN; over Tailscale add it by its Tailscale name or `100.x` address).
   Moonlight shows a **PIN**.
3. In Sunshine Admin, **PIN** tab: enter it and a name for the Mac.

4. In Moonlight's settings (the **gear icon** top right of its main window,
   not the macOS menu), tick **Capture system keyboard shortcuts** and choose
   **in fullscreen**. Stream full screen (**Ctrl+Option+Shift+X**): Cmd then
   reaches Omarchy as Super, including Cmd+Space (Omarchy's launcher, which
   is Raycast's hotkey on the Mac). **Ctrl+Option+Shift+Q** ends the stream,
   **Ctrl+Option+Shift+Z** releases the keyboard.

Repeat steps 2-4 for each new client device.

## 6. Working on dots (optional)

Only needed to commit changes to this repo. `make setup` installs prek
(via mise) and shellcheck (via `omarchy-pkg-add`), then activates the git
hooks:

```sh
cd ~/Work/dots
make setup
```
