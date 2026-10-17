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
| Other keys | Reload = Omarchy's `prefix q` (`prefix r` is rename window), **kept as is** (2026-10-14) although each reload duplicates the `terminal-overrides`/`terminal-features` entries Omarchy's `tmux.conf` appends: harmless, cleared by the next theme switch (dots' theme-set hook resets them) or a server restart, and reported upstream later ([TODO](../TODO.md#omarchy-issues-to-report-found-2026-10-14)); copy-mode `y` = Omarchy's (copy and exit). **Kept `prefix =`** = toggle synchronize-panes (used with tmux-ssh). Dropped: `prefix l` last session (use Omarchy's `Alt+Up`/`Alt+Down`), `prefix e` scrollback→nvim. New bindings carry `-N` notes, so they appear in Omarchy's `prefix ?` help |
| General options | All shared basics already match Omarchy (`tmux-256color`, RGB, focus-events, mouse, base-index 1, renumber, detach-on-destroy off, set-clipboard, vi copy mode, status on top). **Added** `terminal-features ',*:usstyle'` (undercurls, e.g. catppuccin's `SpellBad`; Omarchy lacks it). Kept Omarchy's `escape-time 10`, `history-limit 50000`, `status-interval 5`. Dropped `default-shell zsh` |
| Session persistence | **tmux-resurrect + tmux-continuum, no TPM**: `install.sh` clones them into `~/.local/share/tmux/plugins/` at **pinned commits** (resurrect `cff343c`, continuum `0698e8f`: the masters TPM used to install; both repos have commits after their last tags `v4.0.0`/`v3.1.0`), and `tmux.dots.conf` loads them with `run-shell -b` **last**, because continuum prepends its auto-save to `status-right` at load time (the old catppuccin ordering bug). Continuum also skips auto-saving while another tmux server runs. Defaults kept (manual restore `prefix Ctrl+r`). TPM itself is no longer needed. Compared with herdr (also in Omarchy): both restore layouts after a reboot but not processes; herdr adds AI-agent awareness. Staying on tmux |
| Theme / status bar | **Omarchy's** (named colors follow the Omarchy theme; status shows session, `#I:#W` windows auto-named by directory, COPY/PREFIX/ZOOM + host). catppuccin-tmux (Macchiato) dropped: it would pin tmux to one flavor while nvim and the terminal follow Omarchy, and Omarchy's top bar already shows the time. **Exception (2026-10-14):** the Moonlight theme (Catppuccin) gets its own Macchiato bar (`~/.config/tmux/themes/moonlight.conf`, in dots' tmux package, not the theme), loaded only while that theme is active; see [desktop.md](desktop.md#theme-moonlight-the-mac-studios-catppuccin-look-themesmoonlight) |
| Popups (tmux 3.8) | **Terminal colors**, as on tmux 3.7: tmux 3.8 (the Mac's Homebrew tmux; Omarchy has 3.7c) added built-in themes that draw popups (sesh's picker, `prefix ?`) as a dark grey box with a light border (`popup-style bg=themedarkgrey,…`). `theme terminal` doesn't undo it (it maps the grey to an ANSI color), so `tmux.dots.conf` sets `popup-style` and `popup-border-style` back to `default`, which is 3.7's value: no change on Omarchy, and it keeps Omarchy's look when Arch ships 3.8. Checked on the work Mac (2026-10-12) |
| Sessions (sesh) | **sesh back** (2026-10-12, see below; dropped 2026-10-03). Omarchy's keys kept: `prefix s` session tree, `Alt+Up`/`Alt+Down` or `prefix P`/`N`, `prefix C` new session in current dir, `prefix R`/`K` rename/kill. **`prefix k` = sesh's built-in picker** (`sesh picker -i` in a `display-popup`), **`prefix L` = `sesh last`**. `prefix k` **overrides Omarchy's** (kill window, no confirmation); `prefix &` (tmux default) still kills a window after asking |
| gitmux | Dropped: installed but never referenced in the old status bar |
| tmux-ssh | Moved to the bin scripts / secrets sections (used by `generate-ssh-config`) |

## sesh: back (2026-10-12)

**History:** dropped on 2026-10-03 for a small custom picker
(`~/.config/tmux/session-picker`: tmux sessions then zoxide directories, in
an fzf popup), for three reasons: Omarchy's own keys cover switching
sessions, sesh is AUR-only on Arch, and the old `sesh.toml` held stale
hand-kept project paths. Brought back on 2026-10-12 because the user is used
to it, and sesh has moved on (checked against its README and releases at
v2.32.0):

- **Install:** mise, `"aqua:joshmedeski/sesh"` in `config-dots.toml`: the
  same version on Omarchy and the Macs, no AUR package (Homebrew has it too,
  but mise keeps both systems alike). mise holds a release back for a few
  days after it's published.
- **`prefix k`: sesh's built-in picker** (`sesh picker -i`, icons; since
  v2.28, which its author now uses instead of fzf bindings: preview,
  number jumps, aliases, removing a row). Not the old `fzf-tmux` binding,
  which still works (it's the README's "tmux + fzf" example) if wanted.
  The custom picker is removed.
- **`prefix L`: `sesh last`.** tmux's own last-session loses track after a
  session is closed while `detach-on-destroy off` is set, and Omarchy sets
  it (sesh's README recommends both; Omarchy also has its other
  recommendation, `x` = kill pane without asking).
- **`sesh.toml`: the preview pane** (2026-10-13, `sesh` package):
  `[tui] preview = true`, `preview_width = 55`, the old fzf binding's
  `right:55%`: live panes for tmux sessions; directories as Omarchy's `ls`
  alias lists them (`[default_session] preview_command`: `eza -lh
  --group-directories-first` with `--icons=always --color=always`, since
  the preview isn't a terminal and `auto` would turn both off; sesh's
  default is plain `ls`); `ctrl+o` toggles it. The popup went from 50%x90% to
  the old binding's 80%x70%: sesh hides the pane when the picker is
  narrower than `preview_min_width` (100 columns). No named sessions and no
  `[[wildcard]]`: a wildcard only adds settings such as a startup command
  when connecting; it doesn't list folders. The list comes from tmux, zoxide
  and configured sessions. No `.local` include: sesh's `import` merges only
  sessions/windows/wildcards and fails on a missing file (configurator.go
  at v2.32.0); its `--config` flag covers a separate config. Omarchy ships
  no sesh config, so it's a plain stowed file.
- **`cache-sesh-dirs`** (bin package, back from the old setup) adds every
  `~/Work/*` project to zoxide (top level; skips hidden folders and
  `~/Work/tries`), so the picker lists them on a fresh machine;
  `install.sh` runs it on both systems, and by hand after cloning.
- Tested on Omarchy and the work Mac: sesh 2.32.0 on PATH (mise shims,
  also in tmux's PATH), the bindings loaded, `cache-sesh-dirs` added
  `~/Work`'s projects.
- **Old config** (archived chezmoi repo `~/Work/dotfiles`): the fzf binding
  in `dotfiles/dot_config/tmux/tmux.conf.tmpl`, `sesh.toml.tmpl`, and
  `dot_local/bin/executable_cache-sesh-dirs.tmpl`.

## Mac notes

`tmux.dots.conf` is shared. Omarchy's Alt-key bindings must work on the
Mac too, so the Mac terminal must send **Option as Meta/Alt** (terminal
section).
