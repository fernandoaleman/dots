# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project will adhere to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once the first version is released.

## [Unreleased]

### Added

- `bash` stow package: Omarchy's stock `~/.bashrc` plus `~/.config/bash/rc`
  with vi mode (Omarchy's inputrc re-applied to the vi keymaps, fzf
  `^R`/`^T` verified), `ll`/`ln`/`mkdir`/`grep`/`path` aliases, and git
  completion for Omarchy's `g` alias.
- `install.sh` bootstrap: installs git and stow, clones to `~/Work/dots`,
  backs up conflicting files, stows packages.
- `docs/decisions/` with the shell-section decisions and a reference of
  what Omarchy's bash setup provides.
- README usage guide, MIT license, and this changelog.
- `AGENTS.md` (imported by `CLAUDE.md`) with rules for any AI agent working
  in this repo, including no commit/PR attribution.
- Dev tooling: prek hooks (whitespace, toml/yaml, typos, committed,
  shellcheck), `.shellcheckrc`, and a `make setup` that installs prek via mise.
