# ~/.bash_profile on macOS (dots). Login shells (Terminal, Ghostty, tmux)
# read this, not ~/.bashrc. On Omarchy this file comes from Arch's bash
# package and only loads ~/.bashrc; the Mac first sets up the environment
# Omarchy's bash defaults expect, which Omarchy's env-bootstrap provides
# there (docs/decisions/macos.md).
[[ -r ~/.config/bash/macos-env ]] && source ~/.config/bash/macos-env
[[ -f ~/.bashrc ]] && . ~/.bashrc
