# dots

My dotfiles for [Omarchy](https://omarchy.org), managed with
[GNU Stow](https://www.gnu.org/software/stow/). A macOS setup that mirrors
Omarchy is planned.

The rule is **Omarchy first**: if Omarchy already provides an alias, package
or config, this repo uses it and only adds what Omarchy doesn't have. Every
keep/drop decision, and what Omarchy does for each one, is recorded in
[docs/decisions](docs/decisions/README.md).

## Usage

### Fresh Omarchy install

Follow the full checklist in [docs/setup/omarchy.md](docs/setup/omarchy.md).
It includes the manual steps (Omarchy menu installs, GitHub login and so on).
The bootstrap itself is:

1. Finish the Omarchy installer and log in.
2. Open a terminal and run:

   ```sh
   bash <(curl -fsSL https://raw.githubusercontent.com/fernandoaleman/dots/master/install.sh)
   ```

   This will:
   - ask for your sudo password once (`omarchy-sudo-keepalive`)
   - install `git` and `stow` (`omarchy-pkg-add`)
   - install 1Password and Chrome with Omarchy's own installers, and make
     Chrome the default browser (skipped when already done)
   - clone this repo to `~/Work/dots` over HTTPS (no SSH keys needed yet)
   - move any real file that would block stow to `<file>.bak.<timestamp>`;
     nothing is deleted
   - stow every package listed in `PACKAGES` in `install.sh`
   - add one line to Omarchy's own config files (such as `~/.bashrc`) so
     they load ours; Omarchy's files are never replaced
     (see [docs/decisions/config-pattern.md](docs/decisions/config-pattern.md))

3. Open a new terminal to load the new shell config.

Re-running `install.sh` is safe: it pulls the latest changes and restows.

### Stow by hand

Every top-level directory with dotfiles (such as `bash/`) is a stow
*package* whose contents mirror `$HOME`. From `~/Work/dots`:

```sh
stow --target ~ bash       # install (symlink) a package
stow --target ~ -R bash    # restow after adding or removing files
stow --target ~ -D bash    # uninstall (remove the symlinks)
stow --target ~ -n -v bash # dry run: show what would happen
```

stow won't overwrite a real file. Move it aside first, or let `install.sh`
do it for you. Packages never contain Omarchy's own config files (such as
`~/.bashrc`); `install.sh` adds a line to those instead.

### Updating

```sh
cd ~/Work/dots
git pull
./install.sh
```

### Switching the remote to SSH

`install.sh` clones over HTTPS. Once SSH keys are installed:

```sh
make ssh
```

## Packages

| Package | Installs | What it does |
|---|---|---|
| `bash` | `~/.config/bash/` (+ one line in Omarchy's `~/.bashrc`) | vi mode, a few aliases and git completion for `g` |

## Repository layout

```
install.sh          bootstrap for a fresh install (curl | bash)
bash/               stow package → $HOME
docs/decisions/     what we kept or dropped, and what Omarchy already does
docs/setup/         step-by-step fresh-install checklists (omarchy.md)
Makefile            dev tasks (make help)
prek.toml           git hooks: whitespace, toml/yaml, typos, shellcheck, commit messages
committed.toml      conventional commit rules
.typos.toml         spell-check config
.shellcheckrc       shellcheck config
```

## Development

```sh
make setup    # install prek (mise) + shellcheck, activate the git hooks
make lint     # run every hook on every file
make help     # list all targets
```

Commit messages follow
[Conventional Commits](https://www.conventionalcommits.org) and are checked
by [committed](https://github.com/crate-ci/committed).

## License

MIT
