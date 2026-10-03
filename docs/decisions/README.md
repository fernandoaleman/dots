# Decisions

What we kept, dropped or changed while moving from the old chezmoi/zsh
dotfiles to this repo, and **what Omarchy already does** for each item.

The Omarchy notes are the important part. They record the behavior we chose
to rely on instead of reimplementing, which is exactly what a future Mac
setup has to recreate to "mirror Omarchy, minimally".

## Guiding rules

- **Omarchy first.** If Omarchy already provides an alias, package or
  config, we use Omarchy's version unless there is a specific reason not to.
- **Bash everywhere.** Omarchy uses bash; the Mac will use bash 5+ too.
- **Nothing ported by default.** Each old module was reviewed on its own.
- **Verify, don't guess.** Every Omarchy behavior below was checked in the
  installed files (`/usr/share/omarchy/...`), man pages or upstream docs.

## Sections

| Section | File |
|---|---|
| Config pattern (how we override Omarchy's configs) + tally | [config-pattern.md](config-pattern.md) |
| Shell (bash) | [shell.md](shell.md) |
| Packages | [packages.md](packages.md) |
