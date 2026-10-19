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

## 2. Slack, Teams and Outlook

`install.sh` installs the Slack and Todoist desktop apps and creates the
Teams and Outlook web apps. Once:

- Open **Slack** and sign in (the browser hands the sign-in back to the
  app); allow notifications in Slack's preferences if it asks. Closing its
  window keeps it running in the tray, so notifications continue.
- Open **Todoist** and sign in (the browser hands it back to the app).
- Sign in to **Teams** and **Outlook**; allow notifications when asked (or
  via the site's permission icon in the app window).

## 3. Chrome: AWS Extend Switch Roles

Sign in to Chrome, then install the **AWS Extend Switch Roles** extension
(Chrome Web Store) and paste its role configuration (daily AWS console role
switching, used with `aws-role-login`).

## 4. Log in to GitHub

`gh` is preinstalled by Omarchy (through mise).

```sh
gh-role-login      # browser sign-in, HTTPS (install.sh offers it too)
```

No `gh auth setup-git` needed: dots' git config already uses gh for GitHub
over HTTPS. GitHub stays on HTTPS (no SSH key for GitHub;
[git.md](../decisions/git.md)).

## 5. Work Claude Code plugins

Not automated (for now): after `gh auth login` (the marketplace repo is
private), add the work plugin marketplace and install the work plugins
from it, user-wide: `claude plugin marketplace add <org>/<repo>`, then
`claude plugin install <plugin>@<marketplace> --scope user` for each (or
`/plugin` inside Claude Code). The names are kept out of this public repo;
the old chezmoi script (`run_once_after_35-setup-claude-plugins`) or another
machine's `claude plugin list` has the list.

Public plugins, same way: `claude plugin marketplace add cloudflare/skills`,
`claude plugin install cloudflare@cloudflare --scope user` (its MCP servers
ask you to sign in on first use). From Anthropic's marketplace (already
known to Claude Code): `claude plugin install gopls-lsp@claude-plugins-official
--scope user` and `lua-lsp@claude-plugins-official` (their language servers
come from `install.sh`).

Same on the Mac.

## 6. Remote desktop (Sunshine) for your Mac

`install.sh` installs and starts Sunshine. Then, once:

1. Open **Sunshine Admin** at `https://localhost:47990` in the browser. It
   uses a self-signed certificate: accept the warning for that site only.
   Create the admin username and password (store them in 1Password).
2. On the Mac, open **Moonlight** and add this machine by its **Tailscale
   address** (`tailscale ip -4`; `100.121.13.36` today), which works at home
   and away. Macs keep Tailscale DNS off, so the name won't resolve there
   ([macos.md](macos.md)). Moonlight shows a **PIN**.
3. In Sunshine Admin, **PIN** tab: enter it and a name for the Mac.

4. In Moonlight's settings (the **gear icon** top right of its main window,
   not the macOS menu), tick **Capture system keyboard shortcuts** and choose
   **in fullscreen**. Stream full screen (**Ctrl+Option+Shift+X**): Cmd then
   reaches Omarchy as Super, including Cmd+Space (Omarchy's launcher, which
   is Raycast's hotkey on the Mac). **Ctrl+Option+Shift+Q** ends the stream,
   **Ctrl+Option+Shift+Z** releases the keyboard.

Repeat steps 2-4 for each new client device.

## 7. Syncthing (Obsidian vaults)

`install.sh` installs and starts Syncthing and sets up the vaults
(`~/Obsidian/Work` and `Personal`, each with its `.stignore`, staggered
versioning; [packages.md](../decisions/packages.md#syncthing-obsidian-vaults-across-machines)).
By hand, in its web UI (`http://localhost:8384`):

1. Settings > GUI: set a username and password (store it in 1Password).
2. **When a Mac first runs `install.sh`**, Omarchy shows *"New device"*:
   **Add Device** (address `tcp://<its Tailscale IP>:22000, dynamic`). Then
   accept its folders: **Share** `obsidian-work` with every Mac, and
   `obsidian-personal` only with the Mac Studio (never the work laptop).
   Also **Share** `claude-projects` and `agents` (coding agents'
   sessions, memory, instructions, skills;
   [agents.md](../decisions/agents.md#sync-between-machines-syncthing))
   with every Mac, once it has run `claude-migrate-projects --apply`.
3. In Obsidian: Open folder as vault > `~/Obsidian/Work` (and `Personal`).

Never share `~/Work` or git repos.

## 8. Working on dots (optional)

Only needed to commit changes to this repo. `make setup` installs prek
(via mise) and shellcheck (via `omarchy-pkg-add`), then activates the git
hooks:

```sh
cd ~/Work/dots
make setup
```
