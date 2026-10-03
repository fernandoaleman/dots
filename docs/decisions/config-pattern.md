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

`omarchy-system-factory-reset` is different: it wipes all of `/home`, so
everything is rebuilt with `install.sh`.

## Tally

Every Omarchy-owned config file we customize. **If "no include" ends up
outnumbering "include", revisit switching everything to pattern A.**

| Config | Omarchy's file (kept real) | Our stowed file | Include mechanism | Pattern |
|---|---|---|---|---|
| bash | `~/.bashrc` | `~/.config/bash/rc` (+ `aliases`, `completions`, `inputrc`) | yes: `source` | C |

**Totals: include 1, no include 0.**
