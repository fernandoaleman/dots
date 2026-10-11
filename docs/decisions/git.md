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
warns if a `~/.gitconfig` exists. Defaults (2026-10-11): `DOTS_GIT_NAME` /
`DOTS_GIT_EMAIL` in `lib/dots.sh` (already public in this repo's commits)
are pre-filled in the prompt (`gum input --value`; Enter accepts) and set as
is when there's no terminal. Trade-off: anyone running this `install.sh`
unchanged would get this identity.

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
include line and prompts for identity (the Mac has no installer prompt;
same function as on Omarchy, `check_git_identity`). Identity on the work
MacBook (2026-10-11): the same as Omarchy, `fernandoaleman@mac.com`
everywhere (no separate work email, no `includeIf`). `gh` comes from
Omarchy's mise launcher ([macos.md](macos.md#mise-omarchys-setup-plus-dots-tools));
the credential reset keeps Homebrew git's `osxkeychain` from competing for
GitHub (checked: `credential.helper` stays `osxkeychain` for other hosts,
github.com uses `gh auth git-credential`).

### GitHub over HTTPS on every machine, `gh-role-login` (decided 2026-10-11)

No SSH key for GitHub anywhere: one way to work on every machine, and each
machine holds its own revocable `gh` token instead of one shared key.
Commits aren't signed, so a credential is only needed to push and pull;
`credential.https://github.com.helper` (`gh auth git-credential`, in
`config.dots`) supplies `gh`'s token.

- **Sign-in:** `gh-role-login` ([bin.md](bin.md)) as the profile allows:
  the **work** machine gets a fine-grained token limited to the company
  organization (it can't reach personal repos; one organization, no
  Packages, so it covers admin work through `gh`), **personal** machines the
  normal browser sign-in. Not a classic token for the personal machines: a
  fine-grained token has one owner (user or organization), a classic one is
  broad and long-lived, and the browser sign-in happens once per machine.
- **URLs:** `install.sh` adds `url."https://github.com/".insteadOf` for
  `git@github.com:` and `ssh://git@github.com/` to the machine's own
  `~/.config/git/config` on every profile: repos synced from another machine
  (and dots' old `git@` remote on Omarchy) go over HTTPS without editing
  them. `install.sh` no longer switches remotes to SSH; `make ssh` is left
  for anyone who wants it.
- The personal SSH key comes off GitHub once HTTPS works on every machine
  ([secrets.md](secrets.md)).
