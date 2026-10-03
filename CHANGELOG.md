# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project will adhere to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once the first version is released.

## [Unreleased]

### Added

- `bash` stow package: `~/.bashrc.dots`, loaded by one line that
  `install.sh` adds to Omarchy's own `~/.bashrc` (never replaced), with vi
  mode (Omarchy's inputrc re-applied to the vi keymaps, fzf
  `^R`/`^T` verified), `ll`/`ln`/`mkdir`/`grep`/`path` aliases, and git
  completion for Omarchy's `g` alias.
- `git` stow package: `~/.config/git/config.dots` (loaded by one line in
  Omarchy's `~/.config/git/config`) with `fetch.prune`, `zdiff3` conflict
  style and gh as the GitHub HTTPS credential helper; a lean global
  `~/.config/git/ignore`. `install.sh` asks for the git name/email if unset.
- `nvim` stow package on top of Omarchy's LazyVim: `blink.dots.lua`
  (super-tab completion plus Enter and `<C-j>`/`<C-k>`) and
  `grug-far.dots.lua` (`<A-h>`/`<A-i>` hidden/ignored toggles).
  `install.sh` merges our LazyVim Extras (sidekick plus language Extras)
  into Omarchy's `lazyvim.json`.
- bash: vi command-mode indicator `[vim]` in the prompt (readline
  `show-mode-in-prompt`); starship stays Omarchy's.
- `tmux` stow package on top of Omarchy's tmux: `tmux.dots.conf` (prefix2
  off, vim-tmux-navigator `Ctrl+h/j/k/l` with its nvim half, `prefix =`
  synchronize panes, undercurl support, resurrect + continuum loaded last)
  and a `prefix k` session picker (sessions + zoxide directories).
  `install.sh` clones resurrect and continuum at pinned commits (no TPM).
- `docs/guides/troubleshooting.md` ("it used to do X": dropped behaviors
  and how to re-enable each) and `docs/guides/ruby.md` (ruby-lsp, plus
  legacy Ruby 2.x projects).
- Optional machine-local `.local` layer loaded last by every `.dots` file
  (`~/.bashrc.local`, `~/.config/git/config.local`), ignored by git.
- `install.sh` bootstrap: asks for sudo once (`omarchy-sudo-keepalive`),
  installs git and stow (`omarchy-pkg-add`), installs 1Password and Chrome
  and makes Chrome the default browser with Omarchy's own installers
  (skipped when already done), clones to `~/Work/dots`, backs up
  conflicting files, stows packages, and adds include lines to Omarchy's
  config files.
- `docs/decisions/`: config pattern (Omarchy's files stay real; ours are
  included, named `<name>.dots[.ext]`) with an include/no-include tally, shell and package decisions, with a reference of what
  Omarchy already provides.
- `docs/setup/omarchy.md`: fresh-install checklist of the steps that can't
  be scripted.
- README usage guide, MIT license, and this changelog.
- `AGENTS.md` (imported by `CLAUDE.md`) with rules for any AI agent working
  in this repo, including no commit/PR attribution.
- Dev tooling: prek hooks (whitespace, toml/yaml, typos, committed,
  shellcheck; file checks run once, at pre-commit), `.shellcheckrc`, and `make setup`, which installs prek (via
  mise) and shellcheck (via `omarchy-pkg-add` or brew).
