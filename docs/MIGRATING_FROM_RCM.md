# Migrating from rcm

dots is independent of thoughtbot/rcm. It does not load or execute `.rcrc`.
Migration is manual. Preview with `dots up --dry-run`; Milestone 1 can install
missing links with `dots up` after the preview is reviewed.

Before making your own configuration changes, back up your existing
configuration and dotfiles using your usual backup process.

For example, transfer this rcm source order:

```sh
DOTFILES_DIRS="$HOME/dotfiles-local $HOME/dotfiles"
```

into TOML:

```toml
dotfiles_dirs = ["~/dotfiles-local", "~/dotfiles"]
target_dir = "~"
excludes = [".git", "README.md", "LICENSE"]
warn_duplicates = true
```

Create the TOML file yourself at `~/.config/dots/config.toml`, beneath an
absolute `XDG_CONFIG_HOME`, or at a location selected by `--config`.
The first source directory retains highest priority.

Existing `gitconfig` and `.gitconfig` sources map to `.gitconfig`.
Both `config/nvim/init.lua` and `.config/nvim/init.lua` map to
`.config/nvim/init.lua`. You do not need to rename all source files.
Dotted variants win only within the same source root.

Convert exclusions deliberately: dots uses literal source-root-relative
paths, applies them before precedence, and prunes excluded directories.
rcm glob patterns, shell expressions, tags, host selection, and hooks
are not automatically translated or executed.

Validate the result:

```sh
go run . --config /path/to/config.toml up --dry-run
```

Inspect every planned link, warning, and conflict. Existing links to a
different source remain conflicts. No ownership-based relinking is available.
Do not run `rcdn`, unlink existing files, or switch management based solely on
a successful preview. After reviewing a conflict-free plan, run `dots up`
with the same configuration to create only missing links. Preflight errors
prevent destination changes; actual commands may initialize private ownership
state. Runtime failures retain completed work without rollback.
Existing rcm, M1/M2, and manually created links are not adopted: `OK` means
correctness, not ownership. Only new creations receive registry records.
`down` removes eligible recorded links, so it cannot undo an entire legacy
installation. See [ownership and recovery](OWNERSHIP.md) before using removal.
See the README installation safety section before switching management.

For an existing regular dotfile, the closest dots equivalent to `mkrc` is:

```sh
dots add --dry-run ~/.zshrc
dots add ~/.zshrc
```

With multiple configured roots, explicitly select one with `--root DIR`.
Unlike broad import tools, dots imports one current-user-owned, single-link
regular file at a time and refuses symlinks, directories, occupied source slots
and cross-filesystem relocation. It preserves resolver naming and exclusions,
relocates without replacement, then installs and records a new symlink.
Preview creates nothing; it does not authorize execution. Fresh installations
need no setup command. Failed imports retain evidence and may leave the data
at the repository path; inspect with `dots recover`, never manually edit registry
JSON or assume automatic rollback. See [import usage](../README.md#importing-an-existing-dotfile).
