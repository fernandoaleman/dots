# Config pattern

Decided 2026-10-02. How dots customizes config files that Omarchy owns.

## Pattern C: Omarchy's file stays, ours is included

1. **Omarchy's file stays a real file.** dots never stows over it, so
   Omarchy, its update migrations and tools like `git config --global` or
   `gh auth setup-git` keep editing it normally, and none of their writes
   land in this repo.
2. **`install.sh` appends one line** to it (only if missing) that loads our
   stowed file. It goes last, so our settings override Omarchy's.
3. **Only our own file is stowed**, and it holds only our changes.

### Naming: `.dots`

The file we include is named after the file that includes it, plus
`.dots`, so it's obvious which Omarchy file it belongs to and that it
comes from this repo. Several included files can then sit in one directory.

- **Has an extension:** always put `.dots` before it, for consistency
  (and so loaders and editors still see the real extension):
  `tmux.conf` includes `tmux.dots.conf`, `hyprland.lua` includes
  `hyprland.dots.lua`, `test.txt` → `test.dots.txt`.
- **No extension:** append `.dots`: `~/.config/git/config` includes
  `~/.config/git/config.dots`.
- A hidden file's leading dot is not an extension: `~/.bashrc` includes
  `~/.bashrc.dots`.
- **Exception: where the tool gives dots a meaning, use hyphens.** mise
  `conf.d`: a dotted suffix is becoming an *environment selector*
  (`config.dots.toml` would only load with `MISE_ENV=dots`; dotted names
  are deprecated, removed in mise 2027.8.10), so ours is
  `conf.d/config-dots.toml` and the local one `conf.d/config-local.toml`
  (`.gitignore` also excludes `*-local.toml`).
- Only the included **entry** file follows this rule; anything it loads in
  turn can live wherever makes sense (e.g. `~/.config/bash/aliases`).

Not `.local` for these: by convention that means machine-local and
uncommitted (`settings.local.json`, `.env.local`), which is what the next
layer is for.

### Machine-local layer: `.local`

The last line of every `.dots` file includes a `.local` file **if it
exists**, for one-off changes on a single machine that don't belong in the
repo. Load order:

> Omarchy's file → `.dots` (shared, in the repo) → `.local` (this machine)

Settings in Omarchy's real file load *before* `.dots`, so dots would win
over them; a `.local` file loads last and wins over both.

- Same naming rule: `config.local`, `.bashrc.local`, `tmux.local.conf`.
- Omarchy's file still gets only one added line; the `.local` include
  lives inside our `.dots` file.
- Not every format skips a missing include; each gets checked (tally
  column below) and uses its optional/quiet form if needed.
- `.gitignore` excludes `*.local` and `*.local.*`, so a `.local` file
  created inside a stowed (symlinked) directory can never be committed.

Fallbacks:

- **The format has no include mechanism:** stow a full-file override
  (pattern A: Omarchy's template plus our changes) and note it in the
  tally below.
- **Not an Omarchy file** (a tool Omarchy doesn't configure): stow ours.

`~/.gitconfig` is not used unless absolutely necessary, and then only
temporarily.

### Why not stow Omarchy's file (patterns A and B)

Omarchy's own guide calls `~/.config/` user configuration that is safe to
edit, and a package upgrade never touches it. But `omarchy update` runs
one-time **migrations**, and 32 of the 106 shipped so far make small,
guarded edits to files in `~/.config` (none to git or bash). Tested on a
symlink:

| Migration write method | Effect on a stowed symlink |
|---|---|
| `sed -i`, `mv tmp file` | symlink **replaced by a real file**; repo copy unchanged (silent drift) |
| `>`, `>>`, `cp -f` (also `omarchy refresh`) | writes **through into the repo file** |

Pattern C sidesteps both: migrations edit Omarchy's real file as designed,
and our stowed file is one Omarchy doesn't know about.

A full-file override (pattern A) would also let other tools' writes land
in the repo: `git config --global` (identity), `gh auth setup-git`
(credential helper) and `omarchy refresh` (`cp -f`) all write through a
stowed symlink. With pattern C they stay in Omarchy's real local file.

`omarchy-system-factory-reset` is different: it wipes all of `/home`, so
everything is rebuilt with `install.sh`.

## On the Mac

The same model, with no Omarchy files stored in dots: when the Mac needs an
Omarchy-owned config file (e.g. `~/.config/git/config`), its `install.sh`
downloads Omarchy's template from `github.com/omacom/omarchy` at a
**pinned release tag**, copies it into place once if missing, and then adds
the same include line. The pin is bumped deliberately.

## Post-update hook and `make doctor`

`omarchy update` runs every file in `~/.config/omarchy/hooks/post-update.d/`
(except `*.sample`) right after its migrations. dots stows
**`dots.hook`** there (package `omarchy`), which runs `dots_doctor` from
`lib/dots.sh`:

1. **re-adds missing include lines** (e.g. after `omarchy refresh tmux`),
2. reports dots files in `$HOME` **no longer linked** into the repo (a
   migration's `sed -i` / `mv` replaced the link),
3. reports **uncommitted changes** in `~/Work/dots` (a migration's `>>` /
   `cp -f` wrote through a link, or your own unfinished edits).

Silent apart from one line when all is well; otherwise details in the
update terminal plus one Omarchy notification. `make doctor` runs the same
checks any time; `install.sh` runs them at the end.

`lib/dots.sh` is the single source for `PACKAGES` (what gets stowed) and
`INCLUDES` (each Omarchy file and the line that loads our `.dots` file),
shared by `install.sh`, the hook and `make doctor`.

## Stow without folding

`install.sh` runs `stow --no-folding` (changed 2026-10-04, found by the
clean-install test): Stow creates real folders and links only our files.
With folding, a folder that doesn't exist yet on a fresh machine (there:
`~/.ssh`) becomes a single link into the repo, and everything later written
into it (private keys, `known_hosts`) lands inside the git working tree.
`install.sh` also creates `~/.ssh` (700) before stowing, and `make doctor`
reports any package file that isn't committed. Cost: a new file in a
package needs a re-stow (`install.sh`), for every package alike.

## Tally

Every Omarchy-owned config file we customize. **If "no include" ends up
outnumbering "include", revisit switching everything to pattern A.**

| Config | Omarchy's file (kept real) | Our stowed file | Include mechanism | `.local` (optional include) | Pattern |
|---|---|---|---|---|---|
| bash | `~/.bashrc` | `~/.bashrc.dots` (loads `~/.config/bash/aliases`, `completions`, `inputrc`) | yes: `source` | `~/.bashrc.local`, guarded with `[[ -r … ]]` | C |
| git | `~/.config/git/config` | `~/.config/git/config.dots` | yes: `[include] path = …` (one line) | `~/.config/git/config.local`; git skips missing includes (tested) | C |
| nvim plugins | `~/.config/nvim/lua/plugins/` (dir) | `lua/plugins/<name>.dots.lua` | yes: lazy.nvim loads every file in the dir | n/a (add a `<name>.local.lua` file; gitignored) | C |
| tmux | `~/.config/tmux/tmux.conf` | `~/.config/tmux/tmux.dots.conf` (+ `session-picker`) | yes: `source-file -q` | `~/.config/tmux/tmux.local.conf` (`-q` skips if missing) | C |
| mise | `~/.config/mise/config.toml` (Omarchy, and `mise use -g` writes it) | `~/.config/mise/conf.d/config-dots.toml` (hyphen: naming exception) | yes: mise loads `conf.d/*.toml` automatically (no line needed). ⚠️ Omarchy's `config.toml` takes **precedence** over conf.d (tested), so ours can add but not override | `conf.d/config-local.toml` (alphabetically later, wins over `config-dots.toml`; tested) | C |
| Hyprland monitors | `~/.config/hypr/monitors.lua` | `~/.config/hypr/monitors.dots.lua` | yes: appended Lua line, `dofile` by path if the file exists | `~/.config/hypr/monitors.local.lua` (same check; tested) | C |
| Hyprland (rules, bindings) | `~/.config/hypr/hyprland.lua` | `~/.config/hypr/hyprland.dots.lua` | yes: appended Lua line, `dofile` by path if the file exists | `~/.config/hypr/hyprland.local.lua` | C |
| LazyVim Extras | `~/.config/nvim/lazyvim.json` | none: `install.sh` merges our list with `jq` | **no** (JSON; LazyVim rewrites it) | `:LazyExtras` changes stay local | scripted merge |

**Totals: include 7, no include 1.**
