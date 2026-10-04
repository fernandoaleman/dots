# Bin scripts

Started 2026-10-04 (in progress). Old source: `dotfiles/dot_local/bin/`
(installed to `~/.local/bin`) plus `tmux-ssh` (downloaded from upstream by
`.chezmoiexternal.toml`). Omarchy puts `~/.local/bin` on `PATH`
(`default/bash/envs`).

Already decided elsewhere: `cache-sesh-dirs` (dropped with sesh,
[tmux.md](tmux.md)); `open-herdr.sh` and the `hdl`/`hdlm`/`hds`/`hsl`
links (Mac phase, [packages.md](packages.md)).

## colortest: dropped

Printed the 7 basic terminal colors as regular, bold and underlined text
with the `tput` commands. **Dropped** (2026-10-04): a debugging toy; Omarchy
has no equivalent (its color commands are theme/bar tools), and colors come
from Omarchy's themes. One-liner in [troubleshooting](../guides/troubleshooting.md).

## create-ansible-docker-image: dropped

Scaffolded a repo for an Ansible test Docker image (`image:tag` argument):
Dockerfile (Ubuntu + Ansible via pip + systemd), a GitHub Actions build and
Docker Hub push, lint configs, README, initial commit. **Dropped**
(2026-10-04): work tooling, not a dotfile, and it hard-codes the
employer's Docker Hub and GitHub orgs (not for this public repo). Its home
is a work repo; until then it stays in the old dotfiles repo.

## td-login: dropped

A wrapper: print `td auth status` when logged in, else run
`td auth login "$@"` (browser OAuth; e.g. `--read-only`) and re-check.
Nothing from 1Password. **Dropped** (2026-10-04): `td auth login` /
`td auth status` do the same; the [setup step](../setup/omarchy.md) and
`install.sh`'s logged-out warning cover it. **Mac:** same `td` commands.
