# Troubleshooting: "it used to do X"

Behaviors the old dotfiles had that were **deliberately dropped** in favor
of Omarchy/LazyVim defaults. Look up the symptom, see why, and use the
snippet to bring it back. Snippets go in the `.dots` file named, or in its
`.local` counterpart for this machine only (see
[config pattern](../decisions/config-pattern.md)).

## Neovim

### Search / file picker doesn't find hidden or git-ignored files

**Why:** LazyVim's pickers (`<leader><space>`, `<leader>ff`, `<leader>/`,
`<leader>sg`) and grug-far (`<leader>sr`) respect `.gitignore` and skip
hidden files by default. The old config searched everything
([nvim.md](../decisions/nvim.md)).

**Quick fix, no config:** toggle while searching:

| Where | Hidden | Git-ignored |
|---|---|---|
| Snacks pickers | `<A-h>` | `<A-i>` |
| Snacks explorer (`<leader>e`) | `H` | `I` |
| grug-far | `<A-h>` | `<A-i>` (added by `grug-far.dots.lua`) |

**Make it the default again** (`lua/plugins/snacks.dots.lua`):

```lua
return {
  "folke/snacks.nvim",
  opts = {
    picker = {
      sources = {
        explorer = { hidden = true },
        files = { hidden = true, ignored = true, exclude = { ".git", "node_modules", ".cache", "__pycache__" } },
        grep = { hidden = true, ignored = true, exclude = { ".git", "node_modules", ".cache", "__pycache__" } },
      },
    },
  },
}
```

### lazygit popup (`<leader>gg`) has no dimmed background

**Why:** the old config added `backdrop = 60` to snacks' `lazygit` window
style; the default has none. Dropped as cosmetic.

**Re-enable** (`lua/plugins/snacks.dots.lua`, merge into the same spec):

```lua
opts = { styles = { lazygit = { backdrop = 60 } } }
```

### `<leader><space>` opens a file finder instead of the last file

**Why:** that's LazyVim's default (Find Files, root dir). The old config
remapped it to the alternate file.

**Last file is on:** `<leader>bb`, `` <leader>` `` or Vim's `<C-^>`.
**Remap** (`lua/plugins/snacks.dots.lua`):

```lua
keys = { { "<leader><space>", "<C-^>", desc = "Toggle last two files" } }
```

### No git blame text at the end of the current line

**Why:** `git-blame.nvim` was dropped; LazyVim's gitsigns does blame on
demand. **Use:** `<leader>ghb` (blame line), `<leader>ghB` (blame buffer),
or `:Gitsigns toggle_current_line_blame`. **Always on**
(`lua/plugins/gitsigns.dots.lua`):

```lua
return {
  "lewis6991/gitsigns.nvim",
  opts = {
    current_line_blame = true,
    current_line_blame_formatter = " <author>, <author_time:%m-%d-%Y %I:%M%p> via <<abbrev_sha>> • <summary>",
  },
}
```

### `<leader>dd` doesn't open lazydocker

**Why:** dropped. Omarchy keeps you out of the `docker` group on purpose,
so a plain `lazydocker` can't reach the Docker socket. **Use:** **Super +
Shift + D** (Omarchy's Docker TUI, via a permission prompt).

### `jk` doesn't leave insert mode

**Why:** dropped mapping. **Re-enable** (`lua/config/keymaps.dots.lua`):
`vim.keymap.set("i", "jk", "<ESC>", { desc = "Exit insert mode with jk" })`

### Completion: Enter/Tab/`<C-j>`/`<C-k>` behave differently than expected

**Why:** kept on purpose: `blink.dots.lua` uses blink's `super-tab` preset
plus Enter, `<C-j>`/`<C-k>`, and turns off LSP signature help on insert
`<C-k>` (use `gK` in normal mode). Delete `blink.dots.lua` for LazyVim's
defaults (Enter accepts, `<C-n>`/`<C-p>`).

### I lost something I yanked earlier / `[y` `]y` `<leader>p` don't work

**Why:** the yanky Extra (yank history) was dropped. **Use:** `<leader>s"`
(register picker; `"0` = last yank, `"1`–`"9` = recent deletes).
**Re-enable:** add `coding.yanky` to `NVIM_EXTRAS` in `install.sh` (or
`:LazyExtras` on one machine).

### Claude Code integration (claudecode.nvim) is gone

**Why:** swapped for the `ai.sidekick` Extra, which runs any AI CLI
(codex, opencode, claude…) inside nvim. Re-enable `ai.claudecode` the same
way as above if needed.

### Go tabs look 2 columns wide (not 4)

**Why:** the old `after/ftplugin/go.vim` was dropped. Neovim's go ftplugin
already uses real tabs (`noexpandtab`, gofmt enforces them), but they
display at LazyVim's `tabstop = 2`. **Re-enable 4-wide**
(`lua/config/autocmds.dots.lua`, loaded from Omarchy's `autocmds.lua`):

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "go",
  callback = function() vim.opt_local.tabstop = 4 end,
})
```

### Spell check flags words like LazyVim, dotfiles, systemd

**Why:** the old personal word list (`spell/en.utf-8.add`, 30 words) was
dropped. Spell check is on in markdown, git commits and text (LazyVim).
**Fix per word:** `zg` on the word adds it to
`~/.config/nvim/spell/en.utf-8.add` (local to this machine). **To sync a
word list across machines:** stow `nvim/.config/nvim/spell/en.utf-8.add`
from dots and gitignore `*.spl` (nvim regenerates it). The old list is in
the archived `fernandoaleman/nvim` repo.

### `.sh.tmpl` / `.zsh.tmpl` files have no syntax highlighting

**Why:** those were chezmoi templates; the `ftdetect` for them was dropped.
`*.sh`, `*.zsh` and Ansible playbooks/roles are detected natively or by the
ansible Extra.

### "ATTENTION: Found a swap file" prompt

**Why:** swap files are on again (Neovim default, for crash recovery). The
prompt only appears after a crash (choose Recover, then Delete), or for a
file open in another *non-running* nvim. Swap files live in
`~/.local/state/nvim/swap/`.

### Markdown preview is dark

**Why:** the preview follows the system (Omarchy theme) light/dark
preference; the old config forced light. **Re-enable**
(`lua/plugins/markdown-preview.dots.lua`):

```lua
return { "iamcco/markdown-preview.nvim", init = function() vim.g.mkdp_theme = "light" end }
```

### Neovim colors changed / not Catppuccin Macchiato

**Why:** Omarchy's theme drives nvim (`lua/plugins/theme.lua` is a symlink
to the current theme). Change it with the Omarchy theme switcher; don't pin
a colorscheme in dots.

## tmux

### Ctrl+l doesn't clear the screen / Ctrl+k doesn't delete to end of line (in tmux)

**Why:** vim-tmux-navigator (`tmux.dots.conf`) uses `Ctrl+h/j/k/l` and
`Ctrl+\` to move between panes whenever the pane isn't running vim/nvim or
fzf, so a shell inside tmux never receives them. **Use:** `clear` (or
`Ctrl+l` outside tmux). **Re-enable as prefix keys** (`tmux.dots.conf` or
`tmux.local.conf`):

```tmux
bind C-l send-keys 'C-l'
bind C-k send-keys 'C-k'
```

### A TUI in a tmux pane (opencode, codex, claude, lazygit…) ignores Ctrl+h/j/k/l

**Why:** same as above: only vim/nvim and fzf are detected as "vim", so
tmux takes those keys to switch panes. Use the app's other keys, or add
its process name to `vim_pattern` in `tmux.dots.conf`.

### tmux sessions weren't restored / continuum isn't auto-saving

**Why / checks:** resurrect + continuum load from `tmux.dots.conf` (pinned
clones in `~/.local/share/tmux/plugins/`). Continuum auto-saves every 15
minutes **only if**:

- its hook is in `status-right`: check
  `tmux show -gv status-right | grep continuum_save`. Anything that
  replaces `status-right` *after* continuum loads (a theme, a line in
  `tmux.local.conf`, a future Omarchy migration appending to `tmux.conf`)
  silently removes it. Continuum must stay the last thing loaded.
- only **one** tmux server is running: by design it doesn't save when
  another server exists (e.g. `tmux -L other`).

Restore is manual: `prefix Ctrl+r` (save now: `prefix Ctrl+s`). Saves live
in `~/.local/share/tmux/resurrect/`. Auto-restore on tmux start is off
(continuum's default); `set -g @continuum-restore 'on'` turns it on.

### `prefix k` opens a picker instead of killing the window

**Why:** `tmux.dots.conf` rebinds `prefix k` to the session picker
(replacing sesh). Omarchy's `prefix k` killed the window without asking.
**Kill a window:** `prefix &` (asks first). Restore Omarchy's binding in
`tmux.local.conf`: `bind k kill-window`.

### No sesh / sesh's filters (`Ctrl+a/t/g/x/f`), `Ctrl+d` kill or preview

**Why:** sesh was replaced by the minimal `prefix k` picker
(`~/.config/tmux/session-picker`: sessions + zoxide directories). Ideas to
add later: `fzf --bind 'ctrl-d:…'` to kill a session, `--preview` for a
preview pane. Or go back to sesh: see "sesh: why it was dropped, and how to go back"
in [tmux.md](../decisions/tmux.md).

### Ctrl+b doesn't act as a tmux prefix

**Why:** Omarchy's second prefix (`prefix2 C-b`) was turned off
(`set -g prefix2 None`) so `Ctrl+b` reaches nvim and the shell. The prefix
is `Ctrl+Space`.

### `prefix |` / `prefix -` don't split panes

**Why:** kept Omarchy's split keys: `prefix v` / `Alt+Shift+Enter` (side by
side), `prefix h` / `Alt+Enter` (stacked).

## Shell

### `cd -2` / directory stack, `autocd`, `**` globs don't work

**Why:** zsh-only features; dropped in [shell.md](../decisions/shell.md).
Use zoxide (`cd <partial-name>`). For `**`: `shopt -s globstar`; for
typing a directory name to cd: `shopt -s autocd` (`~/.bashrc.local`).

### A git alias like `gs`, `gco`, `gp` is missing

**Why:** all old git aliases were dropped to learn Omarchy's (`g`, `gcm`,
`gcam`, `gcad`, `ga`/`gd` worktrees). Add individual ones to
`~/.config/bash/aliases`.

### Commands typed in one terminal aren't in another terminal's history

**Why:** Omarchy appends history on shell exit (no live sharing).
**Re-enable** (`~/.bashrc.dots`): `PROMPT_COMMAND+=('history -a; history -n')`

## Git

### Diffs aren't side-by-side / syntax-highlighted

**Why:** delta was dropped (not installed). See [git.md](../decisions/git.md)
for how to bring it back (`omarchy-pkg-add git-delta`; Omarchy's
`BAT_THEME=ansi` makes it follow the theme).

### `git aa`, `git ap`, `git pf` don't exist

**Why:** dropped git aliases. Add to `~/.config/git/config.dots`, e.g.
`[alias] pf = push --force-with-lease`.

## Known Omarchy issues (not caused by dots)

### Lazy reports `monokai-pro.nvim` "fetch failed: Repository not found"

**Why:** Omarchy's `~/.config/nvim/lua/plugins/all-themes.lua` (seeded from
`omarchy-nvim` 2026.8.13) points at `gthelding/monokai-pro.nvim`, which no
longer exists on GitHub. Omarchy fixed its package source on 2026-09-21
(`omacom/omarchy-pkgs` commit `b2e3469`: *"point monokai-pro at loctvl842
(repo not found)"*); open issues: omacom/omarchy #12369, #12560, #12319,
#10094. Harmless meanwhile: the plugin stays installed from cache, and no
Omarchy 4 theme uses it.

**Fix (applied 2026-10-03 on this machine; Omarchy's file, not dots):**

```sh
sed -i 's|"gthelding/monokai-pro.nvim"|"loctvl842/monokai-pro.nvim"|' \
  ~/.config/nvim/lua/plugins/all-themes.lua
nvim --headless "+Lazy! update monokai-pro.nvim" +qa   # switches the remote
```

`Lazy! install` alone doesn't change an installed plugin's remote; an
update (of just that plugin) does. Not needed once an Omarchy release
ships the fix *and* updates your copy.

### Lazy shows "Breaking Changes" / "updates available"

Not an error: lazy.nvim lists newer versions than the ones pinned in
Omarchy's `lazy-lock.json`. Updating (`:Lazy update`) is optional and
moves you off Omarchy's pinned versions.
