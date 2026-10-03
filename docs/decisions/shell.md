# Shell (bash)

Decided 2026-10-02 on Omarchy (bash 5.3, Omarchy package `/usr/share/omarchy`).

## Summary

| Old zsh module | Decision |
|---|---|
| `.zshenv` (XDG vars, ZDOTDIR) | Dropped, Omarchy/uwsm handles XDG |
| `00-homebrew` | Deferred to Mac |
| `10-color` | Dropped |
| `10-editor` | Dropped, Omarchy's `EDITOR` |
| `10-options` | Dropped |
| `20-history` | Dropped, Omarchy's history |
| `20-keybindings` | **Kept: vi mode** + `^A`/`^E`/`^K` |
| `30-fzf`, `95-fzf-keybindings` | Dropped, Omarchy loads fzf |
| `30-gpg` | Dropped |
| `30-plugins` (autosuggestions, syntax highlighting) | Dropped |
| `40-api-tokens` | Deferred to secrets / 1Password |
| `50-aliases` | Mostly dropped. **Kept:** `ll`, `ln`, `mkdir`, `grep`, `path` |
| `50-shortcuts` | Dropped, `~/Work` + zoxide |
| `90-path` | Dropped, Omarchy's PATH |
| `functions/` | All dropped |
| `completion/` | Dropped. **Added:** git completion for `g` |

What we ended up with is the `bash/` stow package (see [Files](#files)).

---

## How Omarchy's bash is wired

This is the structure to mirror on the Mac.

- `~/.bashrc` is a copy of `/etc/skel/.bashrc`:
  1. sources `/usr/share/omarchy/default/bash/env-bootstrap` (OMARCHY_PATH
     and PATH; needed even for non-interactive shells)
  2. returns early if the shell is not interactive
  3. sources `$OMARCHY_PATH/default/bash/rc`
  4. "Add your own exports, aliases, and functions here." This is the
     *Omarchy way* to customize: add lines below the `rc` source, and they
     override Omarchy's defaults because they load later.
- `~/.bash_profile` just sources `~/.bashrc`.
- `$OMARCHY_PATH/default/bash/rc` sources, **in this order**:
  `envs` → `shell` → `aliases` → `functions` (which loops over `fns/*`) →
  `init`, then `bind -f inputrc` for interactive shells.
- Omarchy has no shell plugin or `conf.d` mechanism. The files under
  `~/.config/omarchy/` (hooks, extensions) are for desktop and system
  events, not the shell.

**Our layout copies that pattern** (named files, explicit order, no numbered
`conf.d`): `~/.bashrc` stays the stock stub plus one line,
`source ~/.config/bash/rc`, and our `rc` sources our own named files.

---

## What Omarchy provides (and we rely on)

### Environment (`default/bash/envs`, `env-bootstrap`)

| Thing | Omarchy behavior | Source |
|---|---|---|
| `EDITOR` | `omarchy-launch-editor --inline`, which reads `~/.local/state/omarchy/defaults/editor` (set via `omarchy-default-editor`); falls back to **nvim** | `envs`, `/usr/share/omarchy/bin/omarchy-launch-editor` |
| `SUDO_EDITOR` | `$EDITOR` | `envs` |
| `VISUAL` | **Unset on purpose.** Tools then fall back to `EDITOR`; setting it would bypass Omarchy's editor choice | — |
| `BROWSER` | `omarchy-launch-browser` | `envs` |
| `BAT_THEME` | `ansi` (follows the terminal theme) | `envs` |
| `MANPAGER` | `sh -c 'col -bx \| bat -l man -p'`, with `MANROFFOPT=-c` | `envs` |
| PATH: `~/.local/bin` | **Appended (last)**, so system binaries win. The safety choice; the old zsh setup prepended it | `env-bootstrap` |
| PATH: mise shims | `~/.local/share/mise/shims` appended | `env-bootstrap` |
| PATH: dedupe | Every addition is guarded with `case ":$PATH:"` | `env-bootstrap` |
| XDG base dirs | `XDG_CONFIG_HOME=~/.config`, `XDG_DATA_HOME=~/.local/share`, `XDG_CACHE_HOME=~/.cache`, `XDG_STATE_HOME=~/.local/state`. Set by **uwsm** for the Hyprland session, *not* by bash. Unset in SSH/TTY shells, where programs fall back to the same paths per the XDG spec | `/usr/lib/uwsm/prepare-env.sh` lines 166–172 |

### Shell behavior (`default/bash/shell`)

| Thing | Omarchy behavior |
|---|---|
| History file | `~/.bash_history` (bash default) |
| History size | `HISTSIZE=HISTFILESIZE=32768` |
| Duplicates | `HISTCONTROL=ignoreboth` (skip lines starting with a space, and repeats of the previous command) |
| Writing | `shopt -s histappend`: appended **on shell exit**, not shared live between terminals |
| Completion | Sources `/usr/share/bash-completion/bash_completion` |
| `extglob` | On in interactive shells, enabled by bash-completion (`bash_completion:47`), not by Omarchy directly |
| Unmatched globs | Passed through literally (`failglob`, `nullglob` off; bash default) |
| `set +h` | Command hashing off (for mise) |

### Readline (`default/bash/inputrc`)

- **emacs** editing mode by default.
- ↑/↓ = `history-search-backward/forward` (prefix search).
- Tab = `menu-complete`, Shift-Tab = `menu-complete-backward`, with
  `menu-complete-display-prefix on`.
- `completion-ignore-case on`, `show-all-if-ambiguous on`,
  `mark-symlinked-directories on`, `colored-stats on`, and so on.
- Loaded with `bind -f` and no keymap, so it **only applies to the emacs
  keymap**. That matters for vi mode (see below).

### Tool init (`default/bash/init`)

In order: `mise activate bash`, `starship init bash` (interactive, non-dumb
terminal only), `zoxide init bash`, lazy `try` (`~/Work/tries`), fzf
`completion.bash` + `key-bindings.bash` from `/usr/share/fzf/`, and Omarchy's
own `omarchy` command completion.

fzf with no `FZF_DEFAULT_COMMAND` uses its **built-in walker** (defaults
`file,follow,hidden`; skips `.git,node_modules`; does *not* read
`.gitignore`). The key bindings already use `--height 40%`, and `^T`/`Alt-C`
use `--reverse`.

### Aliases (`default/bash/aliases`)

| Alias | Expands to |
|---|---|
| `ls` | `eza -lh --group-directories-first --icons=auto` |
| `lsa` | `ls -a` |
| `lt` | `eza --tree --level=2 --long --icons --git` |
| `lta` | `lt -a` |
| `ff` | `fzf --preview 'bat --style=numbers --color=always {}'` (kitty variant shows images) |
| `eff` | `$EDITOR "$(ff)"` |
| `cd` | `zd`: zoxide-aware cd (plain dir → `builtin cd`; otherwise `z`; no args → `~`) |
| `..` / `...` / `....` | `cd ..` / `../..` / `../../..` |
| `a` | `omarchy-agent --inline` |
| `c` | `opencode --auto` |
| `cx` | clear screen + `claude --permission-mode auto` |
| `cy` | `codex --approve-for-me` |
| `d` | `docker` |
| `r` | `rails` |
| `t` | `tmux attach \|\| tmux new -s Work` |
| `h` | `herdr` |
| `ic` / `ix` / `icx` | `tdl c` / `tdl cx` / `tdl c cx` |
| `mup` | `MISE_MINIMUM_RELEASE_AGE=0 mise up` |
| `decompress` | `tar -xzf` (from `fns/compression`) |
| **Git** `g` | `git` |
| **Git** `gcm` | `git commit -m` |
| **Git** `gcam` | `git commit -a -m` |
| **Git** `gcad` | `git commit -a --amend` |

### Functions (`default/bash/aliases` + `default/bash/fns/*`)

| Function | File | What it does |
|---|---|---|
| `n` | aliases | `nvim .` with no args, else `nvim "$@"` |
| `open` | aliases | `xdg-open` detached and silenced |
| `sff <dest>` | aliases | pick a file (newest first) with `ff`, then `scp` it to `<dest>` |
| `zd` | aliases | the `cd` replacement above |
| **`ga <branch>`** | `fns/worktrees` | `git worktree add -b <branch> ../<repo>--<branch>`, `mise trust` it, `cd` into it |
| **`gd`** | `fns/worktrees` | inside a `<repo>--<branch>` worktree: `gum confirm`, then remove the worktree and `git branch -D` |
| `compress <dir>` | `fns/compression` | `tar -czf <dir>.tar.gz <dir>` |
| `rsw <src> <dest>` / `lsw` / `dsw` | `fns/rsyncing` | rsync-on-change watcher: `rsw` starts a background `inotifywait` + `rsync` loop (reusing one SSH connection), `lsw` lists watches, `dsw` stops them |
| `fip <host> <ports…>` / `dip <ports…>` / `lip` | `fns/ssh-port-forwarding` | start / stop / list `ssh -L port:localhost:port` forwards |
| `ssh` | `fns/ssh-reconnect` | wraps `ssh` to reset terminal modes (mouse tracking, alternate screen) left on by a dropped remote tmux/herdr/editor, and to reconnect when a connection drops |
| `tdl <ai> [<ai2>]` / `tds` / `tdlm` / `tsl` | `fns/tmux` | tmux dev layouts (`tdl`: editor + AI agent + terminal panes) |
| `hdl` / `hds` / `hdlm` / `hsl` | `fns/herdr` | herdr layouts (same idea as the tmux ones) |
| `iso2sd <iso> [device]` / `format-drive` | `fns/drives` | write an ISO to an SD card with `dd` / format a drive |

**In bash, an alias beats a function of the same name.** Defining our own
`ga`/`gd`/`g` alias would silently hide Omarchy's.

### Git config (`~/.config/git/config`, written by Omarchy)

Aliases `co`, `br`, `ci`, `st`; `init.defaultBranch = master`;
`pull.rebase = true`; `push.autoSetupRemote = true`;
`diff.algorithm = histogram`; `diff.colorMoved = plain`;
`diff.mnemonicPrefix = true`; `commit.verbose = true`; `column.ui = auto`;
`branch.sort = -committerdate`; `tag.sort = -version:refname`;
`rerere.enabled` + `autoupdate`.
(Recorded here for reference; the git config itself is reviewed in the
packages section.)

### Work directory and per-repo `bin/`

- Omarchy's code directory is **`~/Work`** (not `~/code`):
  `install/user/mise-work.sh` creates `~/Work` and `~/Work/tries`; `t`
  attaches to a tmux session named `Work`; `try` uses `~/Work/tries`.
- That same script writes **`~/Work/.mise.toml`** with
  `[env] _.path = "{{ cwd }}/bin"` and runs `mise trust` on it. Result: in
  **any directory under `~/Work`**, that directory's `./bin` is prepended to
  PATH (tested: in `~/Work/tries/x/sub` it added `…/sub/bin`). This
  replaced the old `.git/safe/../../bin` trick. Note the different trust
  model: no per-repo opt-in, but it only applies under `~/Work`.
- `mise use -g node@latest` is also run there (global Node).

---

## Our decisions, with reasons

### Kept / added

- **Vi mode**, with fzf `^R`/`^T`/`Alt-C` verified working. fzf's
  `key-bindings.bash` binds all three keymaps (`emacs-standard`,
  `vi-insert`, `vi-command`).
  - Omarchy's inputrc only reaches the emacs keymap, so in plain vi mode ↑/↓
    lose prefix search and Tab loses menu-complete. **Fix:** after enabling
    vi mode, re-apply Omarchy's file to each vi keymap with
    `bind -m vi-insert -f "$OMARCHY_PATH/default/bash/inputrc"` (and the
    same for `vi-command`). Nothing is copied, so Omarchy updates still
    flow through.
  - Readline's `$include` does **not** expand environment variables (tested),
    which is why this lives in `rc` (bash) and not in `inputrc`.
  - In vi-insert, `^A`/`^E`/`^K` are `self-insert` by default, so they are
    rebound to `beginning-of-line` / `end-of-line` / `kill-line`.
  - Not ported: `^P`, `^N`, `^Y`, `^Q`, `stty -ixon`. In vi-command mode,
    `v` already opens the line in `$EDITOR`.
- **Aliases:** `ll='lsa'` (muscle memory; follows Omarchy's `lsa`),
  `ln='ln -v'`, `mkdir='mkdir -pv'`, `grep='grep --color=auto'` (the old
  `always` leaks escape codes into pipes; tested), and `path`.
- **Git completion for Omarchy's `g`:** Omarchy gives `g` no completion
  spec, so `g <Tab>` only completed filenames. We use git's own helper
  (`/usr/share/bash-completion/completions/git` documents `__git_complete`
  for aliases): source the file, then `__git_complete g __git_main`.
  Cost is about 3 ms of startup.

### Dropped (and why)

- `.zshenv`: XDG vars come from uwsm with the same values; `ZDOTDIR` is
  zsh-only.
- `10-color`: zsh `colors` is zsh-only; `CLICOLOR`/`LSCOLORS` are BSD `ls`
  only; eza uses `LS_COLORS`/`EZA_COLORS` and its defaults follow the
  terminal theme.
- `10-editor`: Omarchy's `EDITOR` is better (menu-selectable, sets
  `SUDO_EDITOR`).
- `10-options`: `extglob` and literal unmatched globs already match. The
  zsh `autopushd` stack has no bash equivalent short of wrapping `cd`, which
  would conflict with Omarchy's `cd`→`zd`; zoxide covers the use case. Also
  chose **not** to enable `globstar` or `autocd` (bash's `autocd` calls the
  builtin `cd` directly, bypassing `zd`).
- `20-history`: kept Omarchy's settings; no live shared history.
- `30-fzf` / `95-fzf-keybindings`: Omarchy's `init` already sources both;
  the built-in walker is nearly identical to the old `rg --files` command.
- `30-gpg`: GPG isn't used; `/usr/bin/pinentry` picks a GUI prompt under
  Wayland anyway.
- `30-plugins`: zsh-only. The bash alternative, **ble.sh**, replaces
  readline and overrides `bind`, and needs special fzf integration;
  rejected to protect vi mode and fzf.
- `50-aliases`: **all Git aliases dropped** on purpose, to learn Omarchy's
  (`g`, `gcm`, `gcam`, `gcad`, `ga`, `gd`) first; individual ones may be
  re-added later. Docker (keep Omarchy's `d`), Bundler/Rails, Terraform, ag,
  chezmoi and `top=btm` were also dropped, as were `cp -iv`, `mv -iv`,
  `e`, `v` and `cat=bat`.
- `50-shortcuts`: `c` conflicted with Omarchy's `c` (opencode); `~/Work` +
  zoxide replace `~/code` + `c <project>`.
- `90-path`: Omarchy already inits mise/starship/zoxide; PATH order kept as
  Omarchy has it (`~/.local/bin` last).
- `functions/`: `g` (was already shadowed by the old `g` alias), the orphan
  `_git_delete_branch`, `change-extension` (zsh-only syntax), `envup` (use
  mise `_.file = ".env"` instead), `mcd`.
- `completion/`: `_ag` and `_bundler`/`_rspec` (tools dropped), `_rg` (rg
  ships its own bash completion).

---

## Files

```
bash/                         stow package → $HOME
├── .bashrc                   /etc/skel/.bashrc + `source ~/.config/bash/rc`
└── .config/bash/
    ├── rc                    sources aliases, completions; vi mode + inputrc re-apply
    ├── aliases               ll, ln, mkdir, grep, path
    ├── completions           git completion for `g`
    └── inputrc               set editing-mode vi; ^A/^E/^K in vi-insert
```

---

## Mac notes (for later)

To make the Mac feel like Omarchy with bash 5+:

- The Mac has no `/usr/share/omarchy`. It will need its own copy of the
  pieces above that we rely on: the `rc` chain, the `envs`, `shell`,
  `inputrc` and `init` equivalents, the aliases, and the `fns/` we actually
  use (at least `ga`/`gd` and `n`).
  Our `bash/.bashrc` and `rc` reference `$OMARCHY_PATH` and
  `/usr/share/omarchy/...`, so the Mac needs a different `.bashrc` (or a
  guarded one).
- PATH: append `~/.local/bin` last; recreate the `~/Work/.mise.toml`
  `_.path = "{{ cwd }}/bin"` behavior.
- XDG vars: nothing sets them on macOS; decide whether to export them.
- `00-homebrew` (deferred): `brew shellenv` + `HOMEBREW_NO_ANALYTICS=1`.
- The login shell must be changed to Homebrew's bash 5+.
