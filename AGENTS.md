# Agent instructions for dots

Dotfiles for Omarchy (and later macOS), managed with GNU Stow.

## Rules

- Omarchy first: if Omarchy already provides an alias, package or
  config, use it. Before changing or overriding anything, explain
  Omarchy's behavior vs the proposed change and ask; default to
  Omarchy's way.
- Verify, don't guess: check /usr/share/omarchy, man pages or official
  docs before stating how something works, and cite what you checked.
- Work one decision at a time and ask before acting on anything
  not yet agreed.
- If a needed tool is missing or unauthenticated, ask whether to
  install or authenticate it instead of working around it.
- Install packages with Omarchy's scripts in /usr/share/omarchy/bin
  (omarchy-pkg-add, omarchy-pkg-aur-add, app installers like
  omarchy-install-*), never raw `pacman -S` / `yay -S`.
- If the user did something manually on Omarchy and an Omarchy script
  does the same, add that script (guarded so re-runs skip it) to
  install.sh instead of documenting it as a manual step.

## Repo conventions

- Each top-level directory with dotfiles is a stow package mirroring
  $HOME; list new packages in PACKAGES in install.sh and in README.
- Record every decision, and what Omarchy does, in docs/decisions/.
- Record only manual fresh-install steps that cannot be scripted
  in docs/setup/omarchy.md (later docs/setup/macos.md).
- Add each change to CHANGELOG.md under [Unreleased].
- Commits: Conventional Commits (committed.toml), subject <= 72 chars.
  Run `make lint` before committing.
- Never add attribution: no Co-Authored-By trailers and no
  "Generated with ..." lines in commits or PRs.
- Never commit secrets; they come from 1Password via `op`.
