# Git

Decided 2026-10-02. Old source: `dotfiles/dot_config/git/` in the chezmoi
repo (`config.tmpl`, `gitignore`, `gitmessage`, `git_template/`).

## Layout (pattern C)

| File | Owner | Contents |
|---|---|---|
| `~/.config/git/config` | **Omarchy** (real file, not stowed) | Omarchy's template + your identity + one line added by `install.sh`: `[include] path = ~/.config/git/config.dots` |
| `~/.config/git/config.dots` | dots (stowed) | our overrides, ending with `[include] path = ~/.config/git/config.local` |
| `~/.config/git/config.local` | you, per machine (optional, never committed) | one-off local overrides |
| `~/.config/git/ignore` | dots (stowed) | global gitignore (git's default path) |

Load order: Omarchy → `config.dots` → `config.local`. git skips a missing
include, and the one-line `[include] path = …` form works (both tested).

Checking values: `git config --global --get` **ignores includes** unless
`--includes` is added; plain `git config --get` (what git itself uses)
follows them. Use `git config --show-origin --get <key>` to see which file
a value comes from.

No `~/.gitconfig`: while it doesn't exist, `git config --global` writes to
Omarchy's `~/.config/git/config` (`man git-config`, FILES).

## What Omarchy's git config sets

From `/usr/share/omarchy/config/git/config`, copied once at install:

| Setting | Value |
|---|---|
| aliases | `co = checkout`, `br = branch`, `ci = commit`, `st = status` |
| `init.defaultBranch` | `master` |
| `pull.rebase` | `true` |
| `push.autoSetupRemote` | `true` |
| `diff.algorithm` / `diff.colorMoved` / `diff.mnemonicPrefix` | `histogram` / `plain` / `true` |
| `commit.verbose` | `true` |
| `column.ui` | `auto` |
| `branch.sort` / `tag.sort` | `-committerdate` / `-version:refname` |
| `rerere.enabled` / `rerere.autoupdate` | `true` / `true` |

Omarchy's installer also sets `user.name`/`user.email` from the name and
email you enter (`install/user/git.sh`). No Omarchy migration has ever
edited this file.

Plus the shell aliases (see [shell.md](shell.md)): `g`, `gcm`, `gcam`,
`gcad`, and the worktree functions `ga`/`gd`.

## Decisions

### Identity

Stays in Omarchy's `~/.config/git/config` (the Omarchy way, machine-local,
not in the public repo). `install.sh` prompts once (gum, or `read`) if
`user.name`/`user.email` are unset, writing with `git config --global`, and
warns if a `~/.gitconfig` exists.

### Kept (in `config.dots`)

| Setting | Why |
|---|---|
| `fetch.prune = true` | Drops remote-tracking branches deleted on the remote, so branch lists stay accurate; never touches local branches |
| `merge.conflictStyle = zdiff3` | Conflict markers include the base text, which helps resolve conflicts (by hand or by agents), especially with Omarchy's `pull.rebase = true` |
| `credential.https://github.com.helper` (+ gist): reset, then `!gh auth git-credential` | gh's login for HTTPS, found on PATH so it survives gh upgrades. Replaces `gh auth setup-git`, which hard-codes a versioned mise path. The empty `helper =` resets earlier helpers (on macOS, Apple git's `osxkeychain`). Stays useful after SSH is set up (HTTPS clones). |

### Global ignore

`~/.config/git/ignore`, stowed: git reads it by default (`man gitignore`),
so no `core.excludesFile` is needed. It's not an Omarchy file (Omarchy
ships none). It keeps only personal/OS/editor patterns: `.DS_Store` (kept on
Omarchy too, since such files arrive from Macs), `*.sw[nop]`,
`.byebug_history`, `pry_history`, `**/.claude/settings.local.json`, and
`.env`/`.env.*` with `!.env.example`. One shared file for both OSes (git's
`includeIf` has no OS condition). Project-specific patterns from the old
file (`/log`, `/tmp`, `node_modules/`, `coverage/`, `public/js/` …) were
dropped; they belong in each project's `.gitignore`.

### Dropped

| Old setting | Why |
|---|---|
| `init.defaultBranch = master`, `alias.co`, `alias.st`, `alias.ci = commit -v` | Identical to Omarchy's (`commit.verbose = true` covers `-v`) |
| `color.ui = true` | git default since 1.8.4 (`true` = `auto`) |
| `push.default = current` | Omarchy's `push.autoSetupRemote` does the same and sets upstream tracking |
| `alias.aa`, `alias.ap`, `alias.pf` | Learning Omarchy's set first (same as the shell aliases) |
| `core.autocrlf = input` | git default; line endings belong in each repo's `.gitattributes` |
| delta (`core.pager`, `interactive.diffFilter`, `[delta]`) | Omarchy doesn't use delta; `git-delta` is not installed. If revisited: Omarchy's `BAT_THEME=ansi` makes delta follow the theme. |
| `commit.template` (`gitmessage`) | Commits are mostly made by Claude/Codex; its 50/80 guidance conflicts with the 72-char Conventional Commits rules |
| `init.templateDir` + `prepare-commit-msg` Jira hook | It would prefix agent commits with `ABC-123`, breaking Conventional Commits (`committed`) |
| `rebase.autoSquash = true` | Only helps a `--fixup` + `rebase -i` workflow |
| `diff.colorMoved = zebra` | Kept Omarchy's `plain` |

## Mac notes

`config.dots` and `ignore` are shared as-is. The Mac seeds
`~/.config/git/config` from Omarchy's template (downloaded at a pinned tag,
see [config-pattern.md](config-pattern.md)), then `install.sh` adds the
include line and prompts for identity (the Mac has no installer prompt).
`gh` comes from Homebrew; the credential reset keeps `osxkeychain` from
competing for GitHub.
