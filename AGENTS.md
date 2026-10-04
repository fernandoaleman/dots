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
  $HOME; list new packages in PACKAGES in lib/dots.sh and in README.
- Never stow over an Omarchy-owned config file. Follow
  docs/decisions/config-pattern.md: keep Omarchy's file real, add its
  include line to INCLUDES in lib/dots.sh (install.sh and the post-update
  hook append it when missing),
  and update the include/no-include tally there.
- Name the included file after the file that includes it, with `.dots`
  before the extension if it has one (`tmux.conf` -> `tmux.dots.conf`),
  otherwise appended (`config` -> `config.dots`, `.bashrc` ->
  `.bashrc.dots`). Exception: where a tool gives dots a meaning, use
  hyphens (mise `conf.d`: `config-dots.toml`, `config-local.toml`).
- End every `.dots` file with an include of its `.local` counterpart
  (same naming, e.g. `config.local`) that is skipped when missing; `.local`
  files are machine-local and never committed (.gitignore).
- Don't use ~/.gitconfig unless absolutely necessary.
- Record every decision, and what Omarchy does, in docs/decisions/.
- Record only manual fresh-install steps that cannot be scripted
  in docs/setup/omarchy.md (later docs/setup/macos.md).
- Put how-to procedures (not decisions, not fresh-install steps) in
  docs/guides/.
- When the user raises an idea or follow-up for later (after the
  migration, or when something happens), add it to docs/TODO.md; when
  one is picked up, record the outcome in docs/decisions/ and remove it.
- When the user reports something "not working" or "used to do X", check
  docs/guides/troubleshooting.md first: it lists deliberately dropped
  behaviors with the snippet to re-enable each. Whenever a decision drops a
  behavior the user might miss, add an entry there.
- Add each change to CHANGELOG.md under [Unreleased].
- Releases (semver, 0.x until Omarchy is complete incl. secrets): move
  [Unreleased] to `## [x.y.z] - date` with compare links, commit
  `chore(release): x.y.z`, annotated tag `vx.y.z`, then a GitHub Release
  whose notes are that changelog section. Ask before publishing.
- Commits: Conventional Commits (committed.toml), subject <= 72 chars.
  Run `make lint` before committing.
- Never add attribution: no Co-Authored-By trailers and no
  "Generated with ..." lines in commits or PRs.
- Never commit secrets; they come from 1Password via `op`.
