# Fresh Omarchy setup

Steps for a brand-new Omarchy install, in order. Only steps that **can't**
be scripted are listed here; anything Omarchy has a script for runs
automatically in `install.sh` instead.

## 1. Run the dots bootstrap

```sh
bash <(curl -fsSL https://raw.githubusercontent.com/fernandoaleman/dots/master/install.sh)
```

It asks for your git name and email if the Omarchy installer didn't set
them. Then open a new terminal. See the [README](../../README.md#usage) for
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

## 4. Working on dots (optional)

Only needed to commit changes to this repo. `make setup` installs prek
(via mise) and shellcheck (via `omarchy-pkg-add`), then activates the git
hooks:

```sh
cd ~/Work/dots
make setup
```
