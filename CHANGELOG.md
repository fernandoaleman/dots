# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While in `0.x`, Omarchy sections are still being added; `1.0.0` will mean
Omarchy is complete, including secrets via 1Password.

## [Unreleased]

## [0.1.0] - 2026-10-03

First release: a fresh Omarchy install plus `install.sh` gives a complete,
usable machine (shell, git, Neovim, tmux, mise tools, apps). Desktop tweaks,
bin scripts and secrets come in later releases.

### Added

#### Bootstrap

- `install.sh` (curl-able, safe to re-run): asks for sudo once
  (`omarchy-sudo-keepalive`), installs packages with `omarchy-pkg-add`,
  installs apps with Omarchy's own installers, clones to `~/Work/dots`,
  backs up conflicting files, stows packages, adds include lines to
  Omarchy's config files, installs tmux plugins and mise tools, merges
  LazyVim Extras, asks for the git name/email if unset, and ends with a
  drift check.
- Config pattern: Omarchy's config files stay real files; each gets one
  line that loads our stowed `<name>.dots[.ext]` file, which ends with an
  optional machine-local `.local` include (ignored by git).
- `omarchy` package: a post-update hook (`dots.hook`) that re-adds include
  lines and reports drift after every `omarchy update`; the same checks as
  `make doctor`. `lib/dots.sh` is the shared source of packages, include
  lines and checks.

#### Packages

- `bash`: `~/.bashrc.dots` with vi mode (Omarchy's inputrc re-applied to
  the vi keymaps, fzf `^R`/`^T` kept), a `[vim]` command-mode indicator,
  `ll`/`ln`/`mkdir`/`grep`/`path` aliases, and git completion for
  Omarchy's `g` alias. Starship stays Omarchy's.
- `git`: `config.dots` with `fetch.prune`, `zdiff3` conflicts and gh as
  the GitHub HTTPS credential helper; a lean global `~/.config/git/ignore`.
- `nvim` (on Omarchy's LazyVim): super-tab completion keys, grug-far
  hidden/ignored toggles, vim-tmux-navigator, and LazyVim Extras (sidekick
  plus language Extras).
- `tmux` (on Omarchy's tmux): prefix2 off, nvim-aware `Ctrl+h/j/k/l`
  pane movement, `prefix k` session picker (sessions + zoxide
  directories), `prefix =` synchronize panes, undercurl support, and
  resurrect + continuum at pinned commits (no TPM).
- `mise`: `conf.d/config-dots.toml` with `.ruby-version` support and
  global tools at `latest` (uv, go, terraform, aws-cli, ansible, yamllint,
  yarn, confluence-cli, jira-cli, todoist-cli).
- Extra packages: `wget`, `nmap`.

#### Apps

- 1Password and Chrome (default browser) via Omarchy's installers.
- ChatGPT desktop via `omarchy-install-ai-chatgpt`.
- Slack, Teams and Outlook web apps via `omarchy-webapp-install`; a
  launcher is recreated when its URL changes.

#### Docs and tooling

- `docs/decisions/`: every keep/drop decision and what Omarchy already
  provides; `docs/setup/omarchy.md`: the steps that can't be scripted;
  `docs/guides/`: troubleshooting ("it used to do X") and Ruby.
- README usage guide, `AGENTS.md` for AI agents, MIT license and this
  changelog.
- prek hooks (whitespace, toml/yaml, typos, committed, shellcheck),
  `make setup`, `make lint`, `make doctor`.

[Unreleased]: https://github.com/fernandoaleman/dots/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/fernandoaleman/dots/releases/tag/v0.1.0
