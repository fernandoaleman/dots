# macOS

Started 2026-10-10 (in progress). The Mac phase (dots 1.x): the 16" M3
MacBook Pro, clean macOS 27 install, as a **full mirror** of the Omarchy
setup, so work continues locally if the remote desktop into Omarchy is
blocked (e.g. office IT blocking ports or Tailscale). Manual steps:
[setup/macos.md](../setup/macos.md).

## One `install.sh` for both systems

Decided 2026-10-10: the same curl one-liner on any machine. `install.sh`
detects macOS vs Omarchy; shared steps (stow, secrets from 1Password, tmux
plugins, mise, git identity, doctor) run on both, and only system-specific
steps branch (packages and apps: Omarchy's scripts vs Homebrew; Omarchy-only
steps such as Sunshine, Docker networks, sudoless Docker).
