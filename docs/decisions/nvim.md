# Neovim

Reviewed 2026-10-03 (all items done except the legacy-Ruby follow-up). Old source: the separate repo
[fernandoaleman/nvim](https://github.com/fernandoaleman/nvim) (LazyVim,
cloned over `~/.config/nvim` by the old chezmoi setup). It will be archived
once this review is done.

Every old customization was checked before keeping it: does Omarchy
already do it, does a LazyVim default or Extra cover it, or has upstream
fixed what it worked around? Checked against LazyVim 16.0.0 and Neovim
0.12.5.

## Layout (pattern C)

- `~/.config/nvim` stays **Omarchy's real directory**, seeded once from the
  `omarchy-nvim` package (`/usr/share/omarchy-nvim/config`, a prebuilt
  LazyVim). Omarchy keeps managing it: `lua/plugins/theme.lua` is a symlink
  to the current Omarchy theme, `omarchy-theme-hotreload.lua` recolors nvim
  on theme switches, `plugin/after/transparency.lua`, `all-themes.lua`
  (every theme's colorscheme, lazy-loaded), and `remote_clipboard.lua`.
  Migrations edit it (e.g. `1781587663` added `remote_clipboard.lua` and
  prepended a line to `options.lua` via `mv tmp file`).
- Our additions are stowed `.dots` files from the `nvim` package:
  - plugin tweaks: `lua/plugins/<name>.dots.lua`. lazy.nvim loads **every**
    `lua/plugins/*.lua` by path (`loadfile`, `lazy/core/plugin.lua`),
    alphabetically, so dotted names work (Lua `require` would not).
  - options, keymaps and autocmds would go in `lua/config/<name>.dots.lua`,
    each loaded by one line `install.sh` adds to Omarchy's file. None are
    needed so far.

## Decisions

### `options.lua`: all dropped

| Old | Why |
|---|---|
| `lazyvim_ruby_formatter = "rubocop"`, `lazyvim_python_lsp = "pyright"` | Already LazyVim's defaults |
| `lazyvim_python_formatter = "black"` | Does nothing: no references in LazyVim 16 (the python Extra formats with ruff) |
| `swapfile = false` | Neovim keeps swap files in `~/.local/state/nvim/swap/` (not in projects), deletes them on normal exit, and skips the prompt when another running nvim owns the file. Default kept for crash recovery |
| `fileformats = { "unix", "mac" }` | `"mac"` means classic Mac OS 9 (CR) line endings, not macOS, and dropping `"dos"` breaks CRLF detection. Default `"unix,dos"` kept |
| `lazyvim_ruby_lsp = "solargraph"` | See Ruby below |

### `keymaps.lua`: all dropped

| Old | Why |
|---|---|
| Visual `<` / `>` keep the selection | Already in LazyVim (`keymaps.lua:87-88`) |
| `<leader>dd` → lazydocker | Wouldn't work: Omarchy deliberately keeps the user out of the `docker` group ("root-equivalent"), so plain `lazydocker` can't reach the socket. Use Omarchy's **Super + Shift + D** (`omarchy-launch-docker-tui`, via a polkit prompt). `<leader>d` is LazyVim's debug group |
| `jk` → Escape | Dropped (personal choice) |

### Jinja: all dropped

No more Jinja/Ansible template editing. (For reference: Neovim detects
`.jinja` but not `.j2`/`.jinja2`; LazyVim already indents with 2 spaces;
nvim-lint now ships a `djlint` linter; conform's `jinja = {}` did nothing.)

### Transparency: dropped

Omarchy's `plugin/after/transparency.lua` clears about 45 highlight groups
(a superset) and is re-applied on every theme switch. The old file's extra
`LazyGitFloat` and `TerminalNormal` groups don't exist in any installed
plugin.

### Colorscheme: dropped, Omarchy themes drive nvim

The old config pinned `catppuccin-macchiato`, which would break Omarchy's
theme switching. Its reason (nvim 0.12 bundles a `catppuccin.vim` that
shadows the plugin) is already handled: Omarchy loads `catppuccin-nvim`,
the plugin's own name. Also redundant: disabling tokyonight (Omarchy
lazy-loads all themes), `transparent_background` (Omarchy's transparency),
italic comments (catppuccin default) and the integrations list (catppuccin
now defaults to `auto_integrations = true`). Default underline style kept
(no undercurl). Current Omarchy theme: Catppuccin (Mocha). No custom
Macchiato theme.

### blink.cmp: kept (muscle memory), `lua/plugins/blink.dots.lua`

blink's **`super-tab`** preset (Tab accepts; the old custom `<Tab>` function
was identical to it), plus `<CR>` accepts and `<C-j>`/`<C-k>` move through
the menu. Insert-mode `<C-k>` (LazyVim: LSP signature help) is turned off
via `opts.servers["*"].keys`, the LazyVim 16 location. The old
`opts.keys` form is deprecated. `version = "*"` and `friendly-snippets` were
dropped (already in LazyVim's spec). LazyVim's default would be the `enter`
preset with `<C-n>`/`<C-p>`.

### git-blame.nvim: dropped

LazyVim's gitsigns already does blame: `<leader>ghb` (line), `<leader>ghB`
(buffer), and `:Gitsigns toggle_current_line_blame` for inline blame
(its formatter supports the old template's fields). No always-on blame.

### grug-far: LazyVim default + toggles, `lua/plugins/grug-far.dots.lua`

LazyVim's `<leader>sr` (prefills the current file's extension, respects
`.gitignore`), plus `<A-h>` / `<A-i>` inside grug-far to toggle hidden /
git-ignored files (grug-far's own `toggle_flags` recipe), registered in
`init`. The old file used `config =` without calling `setup()`, which
silently dropped LazyVim's grug-far opts. Hidden files are searched by
default (`prefills.flags = "--hidden --glob !.git/"`, changed 2026-10-04,
see snacks below); git-ignored files are not. Dropped: `<leader>sRa` (visual `<leader>sr` already
prefills the selection), `<leader>sRw` (`:GrugFarWithin` exists),
`<leader>sRc`.

### markdown-preview: dropped

Comes with the markdown Extra (`<leader>cp`). The preview theme follows the
system (Omarchy theme) light/dark preference instead of being forced
light. (The old `config =` also replaced LazyVim's config for it.)

### Hidden files: `snacks.dots.lua` + `neo-tree.dots.lua`

- **Hidden files shown by default** in the file explorer and the files/grep
  pickers (changed 2026-10-04: projects have many dot files, and the old
  "everything hidden" default made files like `.chezmoiscripts/*`
  unfindable). **Git-ignored files stay hidden** (LazyVim's default;
  `node_modules`, `vendor`, `log/`, build output).
- **The explorer is neo-tree, not the Snacks explorer:** Omarchy's
  `lazyvim.json` (`/usr/share/omarchy-nvim/config/lazyvim.json`) enables
  the `editor.neo-tree` Extra, so `<leader>e` is "Explorer NeoTree".
  `neo-tree.dots.lua` sets `filtered_items` `hide_dotfiles = false`,
  `hide_gitignored = true` (neo-tree defaults: both `true`). neo-tree's own
  `H` (`toggle_hidden`) flips `filtered_items.visible`, showing *every*
  filtered item at once (upstream removed `toggle_gitignore`), so it can't
  hide dotfiles that aren't filtered. `neo-tree.dots.lua` rebinds `H` to
  flip `hide_dotfiles` and adds `I` to flip `hide_gitignored` (both rescan
  with neo-tree's `refresh`; tested both ways), matching `<A-h>` / `<A-i>`
  in the pickers.
- Pickers: `snacks.dots.lua` sets `hidden = true` on the `files` and `grep`
  sources; `<A-h>` / `<A-i>` toggle (same keys in grug-far).
- **`.git/` is never shown or searched:** neo-tree `never_show = { ".git" }`
  (*"remains hidden even if visible is toggled to true"*, so `H` doesn't
  reveal it either); the Snacks files/grep sources always exclude it
  (`fd -E .git`, `rg --glob=!.git`, in
  `snacks/picker/source/{files,grep}.lua`); grug-far's flag carries
  `--glob !.git/`.
- The old config also showed ignored files by default: not kept.
- `<leader><space>` stays LazyVim's Find Files; last file is `<leader>bb`,
  `` <leader>` `` or `<C-^>`.
- lazygit `backdrop = 60` dropped (cosmetic).

All re-enable snippets: [troubleshooting](../guides/troubleshooting.md).

### LazyVim Extras: merged into `lazyvim.json` by `install.sh`

`lazyvim.json` is rewritten by LazyVim itself (`:LazyExtras`, migrations,
via `io.open(path, "w")`), so stowing it would write into the repo, and
importing Extras from a `lua/plugins/*.dots.lua` triggers LazyVim's
"order of your lazy.nvim imports is incorrect" check. So Omarchy's real
`lazyvim.json` is kept and `install.sh` adds our Extras with `jq` (union,
never removes). Extras toggled locally with `:LazyExtras` survive.

| Extras | |
|---|---|
| kept | `lang.ruby`, `lang.json`, `lang.yaml`, `lang.toml`, `lang.markdown`, `lang.docker`, `lang.git`, `lang.python`, `lang.go`, `lang.sql`, `lang.terraform`, `lang.ansible` (+ Omarchy's `editor.neo-tree`) |
| swapped | `ai.claudecode` → **`ai.sidekick`** (folke; runs any AI CLI in nvim: codex, opencode, claude…; its "next edit suggestions" need Copilot, the CLI part doesn't) |
| dropped | `coding.yanky` (yank history): LazyVim's `<leader>s"` register picker covers occasional use |

### `after/ftplugin`, `ftdetect`, spell: all dropped

- **ftplugin:** Neovim's own ftplugins apply each language's recommended
  style: python (*"As suggested by PEP8"*: 4 spaces), yaml (2 spaces), go
  (real tabs; gopls/gofmt enforce), toml (LazyVim's 2-space default).
  `autoindent`/`smarttab` are Neovim defaults. Not kept: Go tabs displayed 4
  wide (now 2, LazyVim's `tabstop`), and `textwidth=79` (ruff handles
  Python line length).
- **ftdetect:** `*.sh`/`*.zsh` are native; Ansible paths are covered by the
  ansible Extra's `nvim-ansible`; `*.sh.tmpl`/`*.zsh.tmpl` were chezmoi.
- **spell:** the 30-word personal list was dropped; `zg` adds words locally.

Re-enable snippets: [troubleshooting](../guides/troubleshooting.md).

### vim-tmux-navigator: kept, `lua/plugins/vim-tmux-navigator.dots.lua`

The nvim half of `Ctrl+h/j/k/l` movement across nvim splits and tmux
panes; decided with its tmux half in [tmux.md](tmux.md).

### Ruby: LazyVim default `ruby_lsp`, legacy projects per project

- LazyVim's ruby Extra with its default **`ruby_lsp`**, configured nowhere
  in dots.
- **Ruby 2.x projects** (the 2.7.8 work app) can't run ruby-lsp directly
  (`required_ruby_version >= 3.0`). They get a project-local `.lazy.lua`,
  using ruby-lsp's documented separate dev-tools Gemfile on Ruby 3, with
  formatting kept on the project's own RuboCop (or solargraph only there as
  a fallback). See the **[Ruby guide](../guides/ruby.md)**. Follow-up:
  build and test it when that repo is set up.
- Dropped from the old config:
  - `nvim-lspconfig.lua`: mise and asdf shim paths (lspconfig runs
    `solargraph` from PATH, and mise shims are on PATH). Its
    `util.root_pattern` `root_dir` is **incompatible** with Neovim 0.11+'s
    `root_dir(bufnr, on_dir)` and duplicated lspconfig's `root_markers`
  - `conform.lua`: the mise and asdf rubocop paths, and a `ruby -e` call at
    **every startup** to choose old RuboCop flags. That's only needed for
    2.x projects, so it moves to their `.lazy.lua`

## Mac notes

The `.dots` files are shared. The Mac seeds `~/.config/nvim` from Omarchy's
LazyVim config at a pinned version. It has no Omarchy theme system, so the
Mac must set a colorscheme directly (Mac-phase decision).
