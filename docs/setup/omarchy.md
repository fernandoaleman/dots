# Fresh Omarchy setup

Steps for a brand-new Omarchy install, in order. Only steps that **can't**
be scripted are listed here; anything Omarchy has a script for runs
automatically in `install.sh` instead.

## 1. Omarchy installer

- Enter your **full name** and **email** when asked. Omarchy uses them to
  set your git identity (`/usr/share/omarchy/install/user/git.sh` runs
  `git config --global user.name/user.email`).

  If they were left blank, set them yourself:

  ```sh
  git config --global user.name "Fernando Aleman"
  git config --global user.email "fernandoaleman@mac.com"
  ```

## 2. Run the dots bootstrap

```sh
bash <(curl -fsSL https://raw.githubusercontent.com/fernandoaleman/dots/master/install.sh)
```

Then open a new terminal. See the [README](../../README.md#usage) for
what `install.sh` does.

## 3. Log in to GitHub

`gh` is preinstalled by Omarchy (through mise).

```sh
gh auth login      # GitHub.com → HTTPS → authenticate Git → web browser
gh auth setup-git  # let git use gh for HTTPS pushes
```

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
