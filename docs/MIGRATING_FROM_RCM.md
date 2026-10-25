# Migrating from rcm

dots is independent of thoughtbot/rcm. It does not load or execute `.rcrc`.
Migration is manual; Milestone 0 previews links without installing them.

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
different source remain conflicts. No ownership-based relinking or
installation is available yet. Do not run `rcdn`, unlink existing files,
or switch management based solely on a successful preview. Switch only
after reviewing the results and when safe mutation is available.
