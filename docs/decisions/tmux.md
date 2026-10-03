# tmux

Reviewed 2026-10-03. Old source: `dotfiles/dot_config/tmux/`
in the chezmoi repo (`tmux.conf.tmpl`, `gitmux.yml`) plus TPM plugins.

## Layout (pattern C)

- `~/.config/tmux/tmux.conf` stays **Omarchy's real file** (copied once
  from `/usr/share/omarchy/config/tmux/tmux.conf`; `omarchy-refresh-tmux`
  resets it with a backup). Migrations edit it in place (`1784401744`,
  `1785189600`, and `1781587663` appended an OSC 52 clipboard line).
- `install.sh` appends `source-file -q ~/.config/tmux/tmux.dots.conf`
  (`-q`: *"no error will be returned if path does not exist"*, tmux 3.7c).
  `tmux.dots.conf` ends with `source-file -q ~/.config/tmux/tmux.local.conf`.
  Lines a future migration appends after ours would load after ours.
- Omarchy's theme uses named colors (`blue`, `brightblack`, `default`), so
  tmux follows the terminal's Omarchy theme; `omarchy-theme-set-tmux` syncs
  theme environment variables into running sessions.

## Decisions

| Item | Decision |
|---|---|
| Prefix | `Ctrl+Space` (same in both). **`prefix2 None`**: Omarchy's second prefix `Ctrl+b` would swallow `Ctrl+b` from nvim (page up, blink docs scroll) and bash |
| Splits | Omarchy's: `prefix v` / `Alt+Shift+Enter` (side by side), `prefix h` / `Alt+Enter` (stacked), all in the current pane's directory; `prefix x` / `Alt+Escape` kill. Old `\|` `-` `_` dropped |
| Pane movement | **vim-tmux-navigator** (muscle memory): `Ctrl+h/j/k/l` and `Ctrl+\` move across tmux panes and nvim splits. The tmux half is the plugin README's manual snippet (no TPM); the nvim half is `nvim/.../vim-tmux-navigator.dots.lua` (replaces LazyVim's `Ctrl+h/j/k/l`; LazyVim skips its own maps when a plugin defines them). Omarchy's `Ctrl+Alt+Arrow` still work. No `prefix Ctrl+l`/`Ctrl+k` restore keys (see troubleshooting) |
| Other keys | Reload = Omarchy's `prefix q` (`prefix r` is rename window); copy-mode `y` = Omarchy's (copy and exit). **Kept `prefix =`** = toggle synchronize-panes (used with tmux-ssh). Dropped: `prefix l` last session (use Omarchy's `Alt+Up`/`Alt+Down`), `prefix e` scrollback→nvim. New bindings carry `-N` notes, so they appear in Omarchy's `prefix ?` help |
| General options | All shared basics already match Omarchy (`tmux-256color`, RGB, focus-events, mouse, base-index 1, renumber, detach-on-destroy off, set-clipboard, vi copy mode, status on top). **Added** `terminal-features ',*:usstyle'` (undercurls, e.g. catppuccin's `SpellBad`; Omarchy lacks it). Kept Omarchy's `escape-time 10`, `history-limit 50000`, `status-interval 5`. Dropped `default-shell zsh` |
| Session persistence | **tmux-resurrect + tmux-continuum, no TPM**: `install.sh` clones them into `~/.local/share/tmux/plugins/` at **pinned commits** (resurrect `cff343c`, continuum `0698e8f`: the masters TPM used to install; both repos have commits after their last tags `v4.0.0`/`v3.1.0`), and `tmux.dots.conf` loads them with `run-shell -b` **last**, because continuum prepends its auto-save to `status-right` at load time (the old catppuccin ordering bug). Continuum also skips auto-saving while another tmux server runs. Defaults kept (manual restore `prefix Ctrl+r`). TPM itself is no longer needed. Compared with herdr (also in Omarchy): both restore layouts after a reboot but not processes; herdr adds AI-agent awareness. Staying on tmux |
| Theme / status bar | **Omarchy's** (named colors follow the Omarchy theme; status shows session, `#I:#W` windows auto-named by directory, COPY/PREFIX/ZOOM + host). catppuccin-tmux (Macchiato) dropped: it would pin tmux to one flavor while nvim and the terminal follow Omarchy, and Omarchy's top bar already shows the time |
| Sessions (sesh) | **sesh dropped**; Omarchy's keys kept: `prefix s` session tree (tmux), `prefix L` last session (tmux), `Alt+Up`/`Alt+Down` or `prefix P`/`N`, `prefix C` new session in current dir, `prefix R`/`K` rename/kill. **Added `prefix k` = session picker** (`~/.config/tmux/session-picker`, in a `display-popup`): running sessions first, then zoxide directories; picking a directory creates or switches to the session named after it. Built only from what Omarchy ships (tmux, fzf, zoxide). It **overrides Omarchy's `prefix k`** (kill window, no confirmation), which collided with sesh muscle memory; `prefix &` (tmux default) still kills a window after asking. Minimal for now (no `Ctrl+d` kill, no preview) |
| gitmux | Dropped: installed but never referenced in the old status bar |
| tmux-ssh | Moved to the bin scripts / secrets sections (used by `generate-ssh-config`) |

## sesh: why it was dropped, and how to go back

**What sesh did (old setup):** `prefix k` opened a `fzf-tmux` popup listing
running tmux sessions, configured sessions (`sesh.toml`) and zoxide
directories, with filters (`Ctrl+a` all, `Ctrl+t` tmux, `Ctrl+g` configs,
`Ctrl+x` zoxide, `Ctrl+f` find dirs), `Ctrl+d` to kill a session, and a
preview. Picking an entry switched to or created the session.

**Why it was dropped (2026-10-03):**

- **Omarchy way first.** Omarchy already covers switching (`prefix s`
  tree, `prefix L` last, `Alt+Up/Down`) and creating (`prefix C`) sessions.
  The only missing piece was "pick a directory → land in its session", and
  that takes a few lines of bash using tools Omarchy already ships (tmux
  `display-popup`, fzf, zoxide).
- **One less package.** sesh isn't in the Arch repos (AUR `sesh-bin`), and
  it needs its own config file (`sesh.toml`) with hand-maintained project
  paths. Those were stale: `~/code/...` (now `~/Work`) and old projects.
  zoxide already knows every directory you use.
- **Key collision.** Omarchy binds `prefix k` to *kill window with no
  confirmation*, which is dangerous with sesh muscle memory. The picker
  takes `prefix k` back.

**What replaced it:** `~/.config/tmux/session-picker`, bound to `prefix k`
in `tmux.dots.conf`. It lists running sessions (`●`) then zoxide
directories; a session switches; a directory switches to the session named
after the folder (`.`/`:` become `_`), creating it there if needed.
Not reproduced (yet): the filters, `Ctrl+d` kill, preview.

**Where the old config is:** the archived chezmoi repo
`~/Work/dotfiles` (`fernandoaleman/dotfiles`):

- `dotfiles/dot_config/sesh/sesh.toml.tmpl`: three work project sessions under `{{ .code_dir }}` (`~/code`)
- `dotfiles/dot_config/tmux/tmux.conf.tmpl`: the `bind-key "k" run-shell
  "sesh connect …"` popup with all the fzf bindings
- `dotfiles/dot_local/bin/executable_cache-sesh-dirs.tmpl`: adds every
  `~/code/*` project to zoxide

**To go back:**

1. `omarchy-pkg-aur-add sesh-bin` (add it to `install.sh` to make it stick).
2. Optional `~/.config/sesh/sesh.toml` (stowed in the `tmux` package or its
   own) with `[[session]]` entries, paths under `~/Work`.
3. In `tmux.dots.conf`, replace the picker's `bind … k display-popup …` line
   with the old `bind-key "k" run-shell "sesh connect …"` block (copied from
   the old `tmux.conf.tmpl`). It uses `fzf-tmux`, which ships with fzf.
4. Optionally delete `~/.config/tmux/session-picker`.

## Mac notes

`tmux.dots.conf` is shared. Omarchy's Alt-key bindings must work on the
Mac too, so the Mac terminal must send **Option as Meta/Alt** (terminal
section).
