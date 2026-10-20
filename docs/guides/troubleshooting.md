# Troubleshooting: "it used to do X"

Behaviors the old dotfiles had that were **deliberately dropped** in favor
of Omarchy/LazyVim defaults. Look up the symptom, see why, and use the
snippet to bring it back. Snippets go in the `.dots` file named, or in its
`.local` counterpart for this machine only (see
[config pattern](../decisions/config-pattern.md)).

## Neovim

### Search / file picker doesn't find git-ignored files

**Why:** hidden (dot) files are shown and searched by default
(`neo-tree.dots.lua`, `snacks.dots.lua`, `grug-far.dots.lua`), but
git-ignored files are not (LazyVim's default). The old config searched everything
([nvim.md](../decisions/nvim.md)). `.git/` is never shown or searched.

**Quick fix, no config:** toggle while searching:

| Where | Hidden | Git-ignored |
|---|---|---|
| Snacks pickers | `<A-h>` | `<A-i>` |
| neo-tree explorer (`<leader>e`) | `H` | `I` |
| grug-far | `<A-h>` | `<A-i>` |

**Make ignored files the default too:** set `hide_gitignored = false` in
`lua/plugins/neo-tree.dots.lua`, add `ignored = true` to the `files` and
`grep` sources in `snacks.dots.lua`, and add `--no-ignore` to the grug-far
flags in `grug-far.dots.lua`. Expect `node_modules`, `vendor` and build
output in results.

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

**Why:** `tmux.dots.conf` rebinds `prefix k` to sesh's picker. Omarchy's
`prefix k` killed the window without asking.
**Kill a window:** `prefix &` (asks first). Restore Omarchy's binding in
`tmux.local.conf`: `bind k kill-window`.

### tmux popups are a grey box with a light border (tmux 3.8)

**Why:** tmux 3.8 (Homebrew; Arch still has 3.7c) themes popups with
`popup-style bg=themedarkgrey,fg=themewhite`. **Fixed:** `tmux.dots.conf`
sets `popup-style` and `popup-border-style` to `default` (the terminal's
colors, as in 3.7). A running tmux picks it up with `prefix q`.

### sesh's old fzf popup (`Ctrl+a/t/g/x/f` filters, `Ctrl+d` kill)

**Why:** `prefix k` uses sesh's built-in picker now, not the old
`fzf-tmux` binding ([tmux.md](../decisions/tmux.md#sesh-back-2026-10-12)).
To get the old popup back, put the "tmux + fzf" `bind-key` block from sesh's
README (or the archived `tmux.conf.tmpl`) in `~/.config/tmux/tmux.local.conf`
with `k` as the key.

### sesh picker doesn't list a `~/Work` project

**Why:** the picker lists tmux sessions and zoxide's folders; a project you
haven't visited isn't in zoxide yet. **Fix:** `cache-sesh-dirs` (adds every
`~/Work/*` folder; `install.sh` runs it), or `cd` into it once.

### sesh picker has no preview on the right

**Why:** sesh drops the preview pane when the picker is narrower than
`preview_min_width` (100 columns), e.g. in a half-screen terminal (the
popup is 80% of it), and `ctrl+o` toggles the pane off and on. **Fix:**
press `ctrl+o`, widen the terminal, or lower it in
`~/.config/sesh/sesh.toml` (`[tui] preview_min_width = 80`). If the pane is
missing everywhere, check `~/.config/sesh` is linked (`install.sh` stows the
`sesh` package).

### A dots file was removed, but its link is still in `$HOME`

**Why:** re-stowing removes links to deleted package files only in folders
the package still has; when a whole folder goes, its links stay, pointing
at nothing (seen with `~/.config/tmux/session-picker`, and the Mac's
`~/.config/ghostty/themes/onedark-dark` on 2026-10-19). `make doctor`
reports it while the file is still tracked. **Fix:** re-run `install.sh`,
which now deletes broken links into dots
([config-pattern.md](../decisions/config-pattern.md#stow-without-folding));
for one elsewhere, `find ~/.config ~/.local/bin -xtype l` lists broken
links (on a Mac, `find … -type l ! -exec test -e {} \; -print`).

### Ctrl+b doesn't act as a tmux prefix

**Why:** Omarchy's second prefix (`prefix2 C-b`) was turned off
(`set -g prefix2 None`) so `Ctrl+b` reaches nvim and the shell. The prefix
is `Ctrl+Space`.

### `prefix |` / `prefix -` don't split panes

**Why:** kept Omarchy's split keys: `prefix v` / `Alt+Shift+Enter` (side by
side), `prefix h` / `Alt+Enter` (stacked).

## lazygit

### No file/branch icons in lazygit

**Why:** lazygit runs on its defaults (Omarchy's `config.yml` is empty);
icons are `gui.nerdFontsVersion`, which defaults to off (the old
`showIcons` setting no longer exists). **Re-enable:** `nerdFontsVersion:
"3"` (Omarchy ships Nerd Fonts). lazygit's YAML has no include; multiple
files can be merged with `LG_CONFIG_FILE="a.yml,b.yml"`, but that env var
must be set wherever lazygit starts (shell, nvim `<leader>gg`, launcher).

### Staging selects whole hunks / discarding asks for confirmation

**Why:** lazygit defaults (`useHunkModeInStagingView: true`,
`skipDiscardChangeWarning: false`); the old config flipped both. Press
`a` in the staging view to toggle hunk/line selection.

## Terminal

### Terminal font looks small / text isn't copied on select

**Why:** foot (Omarchy's default) with Omarchy's `~/.config/foot/foot.ini`:
font size 9, no copy-on-select (the old ghostty/alacritty used size 12,
Medium, copy-on-select). Change it in foot's config; foot supports
`include=`, so tweaks could go in a `foot.dots.ini` via the config pattern.

### Omarchy (foot) and the Mac terminal behave differently

**Why:** foot is Linux-only, so the Mac runs a different terminal (likely
ghostty). To make both identical, switch Omarchy to ghostty:
`omarchy-install-terminal ghostty` (Omarchy's installer, also in the menu:
Install → Terminal), then add that command to `install.sh`. See
[packages.md](../decisions/packages.md) (Terminals).

## Hyprland

### A new monitor / laptop screen uses Omarchy's default layout

**Why:** `monitors.dots.lua` matches the desk monitors by description
(model + serial), so other screens fall back to Omarchy's catch-all rule
(scale 1.6, auto position). Add a rule for it: `hyprctl monitors all`
shows its description; put it in `monitors.dots.lua` (shared) or
`monitors.local.lua` (this machine only).

### Omarchy's display scale menu doesn't change the desk monitors

**Why:** the menu edits only the catch-all rule in Omarchy's
`monitors.lua` (and its `GDK_SCALE`). `monitors.dots.lua`, loaded after it,
sets scale 1.6 for each desk monitor by description, so those rules win.
**Fix:** change the scale in `monitors.dots.lua`; to try one on this machine
only, copy its three `hl.monitor` lines into `monitors.local.lua` with the
new scale and positions in scaled pixels (2560 / scale per monitor, e.g.
2048 at 1.25). Delete the local file to go back.

### Super+V types `^V` (or Super+C interrupts) in a terminal window

**Why:** Omarchy's Super+C/V send terminal keys (Ctrl+Insert/Shift+Insert)
only to windows tagged `terminal`, matched by class; a terminal launched
with its own app-id (like dots' `tmux`) isn't, so it gets Ctrl+V/Ctrl+C.
Check with `hyprctl clients -j | jq '.[] | {class, tags}'`. **Fix:** tag
the class in `hyprland.dots.lua`: `o.window({ class = "^name$" }, { tag =
"+terminal" })`. Meanwhile Ctrl+Shift+V / Shift+Insert paste in foot.

### A workspace is on the wrong monitor

**Why:** workspaces created before the rules (or while a monitor was
unplugged) stay where they were created. Focus it and press
Super+Shift+Alt+Arrow, or
`hyprctl dispatch 'hl.dsp.workspace.move({ monitor = "DP-1" })'`; logging
out also resets them.

### The bar skips some workspace numbers (e.g. 6 or 8)

**Why:** Omarchy's workspaces widget always shows 1–5, and 6–10 only while
that workspace exists (it has a window or a monitor is showing it).
Pins don't affect it. It's the same list on every monitor. Kept as
Omarchy's (2026-10-04); always showing 1–9 or per-monitor numbers would
need `omarchy plugin clone omarchy.workspaces` and a custom edit.

### An app opened on a different workspace than expected

**Why:** `hyprland.dots.lua` pins apps: Chrome → 1, tmux terminal → 4,
Slack/Teams → 7, Outlook → 8, Spotify → 9 (matched by window class when it opens).
Windows opened before the rule existed stay put; move one with
Super+Shift+N. Add or change pins in `hyprland.dots.lua` (or
`hyprland.local.lua` for one machine). Find an app's class with
`hyprctl clients`.

### Super+Alt+Return terminal isn't on workspace 4 / other terminals aren't pinned

**Why:** only the tmux terminal from **Super+Alt+Return** is pinned: dots
rebinds that key to Omarchy's same command plus `--app-id=tmux`, which the
rule matches. A terminal from Super+Return is a plain window and opens
where you are (pinning every terminal would be too broad).

### `hyprctl dispatch workspace 2` errors

**Why:** with Omarchy's Lua config, dispatch takes Lua:
`hyprctl dispatch 'hl.dsp.focus({ workspace = "2" })'`.

### CapsLock doesn't start a Compose sequence (é, ©, emoji, name/email)

**Why:** dots made CapsLock a Hyper key for CapsLock+H/J/K/L
([desktop.md](../decisions/desktop.md#capslock-is-hyper-capslockletter-keys-compose-on-right-alt));
Compose is **Right Alt** now (Right Alt, Space, n types your name).
**Bring CapsLock-Compose back:** remove the `kb_options` line in
`hyprland.dots.lua`, then `hyprctl reload` and `omarchy-restart-xcompose`.

### CapsLock still acts as Compose after changing keyboard options

**Why:** fcitx5's virtual keyboard keeps the options it started with
(`hyprctl devices` shows the old ones on `hl-virtual-keyboard-fcitx5`).
**Fix:** `omarchy-restart-xcompose` (`install.sh` does it when they differ).

### A new dots key binding does nothing (e.g. Super+Shift+R)

**Why:** Hyprland didn't reload on its own after `hyprland.dots.lua` (a
link into dots) changed; `hyprctl binds -j` didn't list the new key
(found 2026-10-20). **Fix:** `hyprctl reload`. `install.sh` now does it at
the end when Hyprland is running.

## Slack

### Can't draw on someone's shared screen in a Slack huddle

**Why:** Slack supports drawing on shared screens only in its **desktop
app for Mac and Windows**; not in the browser, not on Linux (also not in
AUR `slack-desktop`). **Use:** the Mac for demos/onboarding that need it.

### Slack notifications stopped

**Why:** check Slack's own notification preferences (and Do Not Disturb)
first. The desktop app keeps notifying from the tray when its window is
closed; if you quit it from the tray icon, it stops until reopened.

### Want the Slack web app back (instead of the desktop app)

Slack moved from a Chrome web app to the native app on 2026-10-17
([packages.md](../decisions/packages.md)). To go back: add
`"Slack|https://<workspace>.slack.com|https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/slack.png"`
to `WEBAPPS` in `install.sh`, drop the `slack-desktop` block and the
web-app removal there, point the workspace 7 rule in `hyprland.dots.lua` at
the web app's class (`chrome-<workspace>.slack.com__-Default`), then
`omarchy-pkg-drop slack-desktop` and re-run `install.sh`. Use the workspace
URL, not `https://app.slack.com`: with several workspaces the latter shows a
picker whose *Launch* link opens a browser tab that tries the `slack://`
link ("Open With… No Apps available"). The web app notifies only while its
window is open.

### Todoist: "Safe Storage Encryption is not available" or `--no-sandbox`

**Symptom:** started from a terminal, Todoist logs *"Safe Storage Encryption
is not available"* (its login token is stored unencrypted), or `ps` shows it
running with `--no-sandbox`. **Cause:** it was started by the package's
launcher (or plain `todoist`) instead of dots' override; Electron doesn't
recognize Hyprland, so it doesn't use gnome-keyring without
`--password-store=gnome-libsecret`. **Fix:** re-run `install.sh` (it rewrites
`~/.local/share/applications/todoist.desktop`), or start it by hand with
`todoist --password-store=gnome-libsecret`. The first start with the keyring
logs a "Failed to decrypt API token" error and asks you to sign in again
(the old token wasn't encrypted); that's expected. The same applies to any
Electron app that keeps secrets: add the flag.

### Want the Todoist web app back

Add `"Todoist|https://app.todoist.com"` to `WEBAPPS` in `install.sh`, drop
the Todoist block and `Todoist` from the web-app removal loop there, then
`omarchy-pkg-drop todoist-appimage appimagelauncher`,
`rm ~/.local/share/applications/todoist.desktop` and re-run `install.sh`.

### A web app was created without an icon / "Could not install the X web app"

**Why:** `omarchy-webapp-install` fetches the site's icon automatically and
fails when the site has none (e.g. Slack workspace subdomains). Give the
entry an explicit icon URL in `WEBAPPS` in `install.sh`
(`"Name|URL|icon URL"`, e.g. from
`https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/<name>.png`).

### No ~/GoogleDrive or ~/iCloudDrive

**Why:** the old rclone mounts weren't set up on Omarchy (by choice). See
the [rclone guide](rclone.md) to bring them back.

### No AWS VPN client

**Why:** dots no longer installs it (the old AUR package is unsupported by
AWS). It's installed separately on Omarchy; see
[desktop.md](../decisions/desktop.md).

### No espanso / `:date`-style expansions or Shift+Alt+Space search

**Why:** espanso was dropped (its config had only stock examples;
[desktop.md](../decisions/desktop.md)). Omarchy's Compose key covers
name/email: CapsLock, Space, `n` / `e` (filled from git by `install.sh`).
Add your own lines to `~/.XCompose`, then `omarchy-restart-xcompose`:

```
<Multi_key> <space> <s> : "Some text"
```

**Bring espanso back:** add `omarchy-pkg-aur-add espanso-wayland` to
`install.sh`, then finish the setup per espanso's Wayland instructions
(https://espanso.org/docs/install/linux/). The old config was stock, so
there's nothing to restore.

## Docker

### `docker ps`: permission denied on `/var/run/docker.sock`

**Why:** sudoless Docker (the `docker` group) only applies after a
**reboot**; until then use `sudo docker …`. Check with
`omarchy-sudo-docker --configured || echo "configured"`. To go back to
Omarchy's default (sudo / polkit): `omarchy-remove-security-sudoless-docker`
([desktop.md](../decisions/desktop.md)).

### Containers can't resolve DNS / VPN hosts on 172.17.x.x unreachable

**Why:** `install.sh` moves Docker's bridge to `172.31.0.1/16` and its
other networks to `192.168.128.0/17` (the VPN routes `172.17.x.x`), which
also moves container DNS ([desktop.md](../decisions/desktop.md)). Check:

```sh
make doctor                              # reports if daemon.json was reset
jq '.bip, .dns' /etc/docker/daemon.json  # "172.31.0.1/16", ["172.31.0.1"]
resolvectl status | head                 # stub listener incl. 172.31.0.1
sudo ufw status | grep docker-dns        # rules to 172.31.0.1
```

Re-run `install.sh` to restore all three. If an `omarchy update` left a
`/etc/docker/daemon.json.pacnew`, merge Omarchy's new keys into
`daemon.json` and re-run `install.sh`. Old compose networks still on
`172.x`: recreate them (`docker network prune`).

**Go back to Omarchy's defaults:** remove the step from `install.sh`, then
`sudo rm /etc/systemd/resolved.conf.d/30-dots-docker-dns.conf`, restore
`daemon.json` (`bip`/`dns` to `172.17.0.1`, drop `default-address-pools`)
and restart `systemd-resolved` and `docker`.

## AWS

### `aws-sso-login` not found / I need an SSO login

**Why:** dropped; daily access is role + MFA (`aws-role-login`). The SSO
profiles live in `~/.aws/config.sso` (one session for all 10 profiles),
which the AWS CLI reads only when told to:

```sh
AWS_CONFIG_FILE=~/.aws/config.sso aws sso login                  # one login, all profiles
AWS_CONFIG_FILE=~/.aws/config.sso aws <command> --profile <name>
```

## Ruby

### `pry` not found / `:checkhealth` warns about the Ruby provider

**Why:** `~/.default-gems` (installed `pry` and `neovim` into every Ruby)
was dropped; mise deprecates it ([dotfiles.md](../decisions/dotfiles.md)).
The Ruby provider warning is harmless (no LazyVim plugin is written in
Ruby). To bring pry back, the current mise ways, in
`mise/.config/mise/conf.d/config-dots.toml`:

```toml
[tools]
"gem:pry" = "latest"   # pry as a standalone command
```

or per project: `gem "pry"` in the `Gemfile`, then `bundle exec pry`.

### RSpec output isn't in documentation format / no slowest-examples list

**Why:** the global `~/.rspec` was dropped
([dotfiles.md](../decisions/dotfiles.md)). Pass flags per run
(`rspec --format documentation --profile 10`) or put them in the project's
`.rspec`. For personal defaults again, add `ruby/.rspec`, one option per
line and **no inline comments** (RSpec treats them as file arguments):

```
--format documentation
--profile 10
```

## SSH

### `ssh` asks for a key passphrase every time

**Why:** dots runs no SSH agent, because the keys have no passphrases
([secrets.md](../decisions/secrets.md)). If a key gets one, enable Omarchy's
installed keyring agent and let ssh add keys to it:

```sh
systemctl --user enable --now gcr-ssh-agent.socket
# in ~/.bashrc.local:
export SSH_AUTH_SOCK="$XDG_RUNTIME_DIR/gcr/ssh"
# in ~/.ssh/config.local (or ssh/.ssh/config under Host *):
#   AddKeysToAgent yes
```

### Connections take minutes to notice a drop

**Why:** something overrides Omarchy's keepalives (15 s x 3). Check with
`ssh -G <host> | grep -E "serveralive|connecttimeout"`; remove
`ServerAliveInterval` from `~/.ssh/config` / `config.local`.

## Tailscale

### On a Mac, nothing resolves with both Tailscale and the AWS VPN connected

**Why:** with "Use Tailscale DNS" on, macOS lets Tailscale replace all DNS
(only `100.100.100.100`), pushing out the AWS VPN's resolvers
([desktop.md](../decisions/desktop.md)). **Fix:** in the Tailscale menu,
turn **Use Tailscale DNS** off (`tailscale set --accept-dns=false`) and keep
Exit Node on None. If the Mac is already stuck: disconnect the VPN and quit
Tailscale (sites come back), then start Tailscale first, then the VPN.
Omarchy isn't affected (`systemd-resolved` keeps each connection's DNS
separate). New Macs: [macos.md](../setup/macos.md) step 4.

### AWS VPN sign-in: "Looks like this code isn't right" (work Mac)

**Why:** the VPN signs in through AWS SSO (the `awsapps.com` start page),
whose MFA is a different device from the **AWS Role Switch** item's (the
IAM console). The work Mac reads only the work 1Password account, so if the
**AWS SSO** login is missing there, the Role Switch code gets copied and
rejected (seen 2026-10-11; the clock was fine). **Fix:** move the AWS SSO
login to the work account in the 1Password app, then sign in with its
username, password and code.

### A Mac shows `ip-10-0-0-30` (or `…compute.internal`) instead of its name

**Why:** with no HostName set, macOS names itself after a reverse DNS
lookup of its IP; with the AWS VPN connected, AWS's resolver answers in its
`ip-a-b-c-d` style. **Fixed:** `install.sh` sets HostName from the model
(`mac-studio`, `macbook-pro`) when it's unset. Two Macs of the same model
would get the same name: set one by hand, `sudo scutil --set HostName
<name>` (install.sh then leaves it). Renaming the Mac in System Settings
doesn't change HostName.

## Remote desktop (Sunshine / Moonlight)

### `omarchy-install-service-sunshine` fails: "Unit sunshine.service does not exist"

**Why:** a known Omarchy 4.0.4 bug: the unit is
`app-dev.lizardbyte.app.Sunshine.service` (`sunshine.service` is only an
alias). `install.sh` doesn't use that installer; it does the same steps
itself ([desktop.md](../decisions/desktop.md)). Re-run `install.sh` instead.

### Sunshine crashes on every login / runs twice

**Why:** Omarchy's installer also adds `o.launch_on_start("sunshine")` to
`~/.config/hypr/autostart.lua`, on top of the systemd unit that already
starts with the session. **Fix:** delete that line; keep
`systemctl --user status app-dev.lizardbyte.app.Sunshine.service`.

### Sunshine Admin shows a certificate warning

Expected: `https://localhost:47990` uses a self-signed certificate. Accept
it for that site in the browser. Don't open it with
`--ignore-certificate-errors` (Omarchy's web app does), which turns off
certificate checks for the whole browser session.

### Moonlight can't find the Omarchy desktop away from home

Add it in Moonlight by its **Tailscale address `100.121.13.36`** (not the
name: Macs keep Tailscale DNS off). Check Tailscale is connected on both
ends, and that the firewall allows Tailscale:
`sudo ufw status | grep omarchy-sunshine` should list `on tailscale0` rules
(re-run `install.sh` if not).

### Moonlight shows the wrong monitor

Sunshine streams `output_name = DP-2` (the middle monitor) and falls back to
a connected one when it's unplugged. To change it, edit `SUNSHINE_OUTPUT`
in `install.sh` (the connector names are in `hyprctl monitors`) and re-run
it.

### Cmd / Super shortcuts don't reach Omarchy through Moonlight

**Why:** macOS hands system-wide hotkeys (Raycast's Cmd+Space, Cmd+Tab) to
their owner. **Fix:** Moonlight's gear icon (main window) > **Capture system
keyboard shortcuts: in fullscreen**, and stream full screen
(Ctrl+Option+Shift+X). Ctrl+Option+Shift+Z releases the keyboard.

### Sunshine's web UI says a new (security) version is available

The installed package comes from Omarchy's repo, which lags upstream (known
advisory, accepted for now; [desktop.md](../decisions/desktop.md)). Update
when Omarchy ships it (`omarchy update`). If it keeps lagging, the patched
upstream build is AUR `sunshine-bin`:
`omarchy-pkg-drop sunshine && omarchy-pkg-aur-add sunshine-bin`.

## macOS

### How do I remove a Mac app completely?

Apps installed by dots come from Homebrew casks: `brew uninstall --cask
--zap <name>` removes the app and the leftover files listed in its cask
(settings, caches), then drop it from the `Brewfile` so `install.sh`
doesn't bring it back. AppCleaner (installed) is the fallback for apps
installed outside dots.

### `brew uninstall --cask --zap` asks for Full Disk Access

**Why:** some casks' zap lists include macOS-protected paths (Logi
Options+: `~/Library/Application Support/com.apple.sharedfilelist/…`), and
deleting those needs Full Disk Access for the terminal. Don't grant it:
it lets the terminal, and everything run in it, read all protected data.
**Fix:** `brew uninstall --cask <name>` (no `--zap`; add `--force` if the
app is already gone), then delete the leftovers you can see. Installer-type
casks (Logi Options+) can also leave apps and package receipts the cask
doesn't track: check `pkgutil --pkgs | grep -i <vendor>`, remove the files
it lists (`pkgutil --files <pkg>`), then `sudo pkgutil --forget <pkg>`.

### `brew bundle` fails: "Refusing to load cask … from untrusted tap"

**Why:** Homebrew 7 only loads formulae and casks from third-party taps
once they're trusted (`Library/Homebrew/trust.rb`; trust lives in
`~/.homebrew/trust.json`). A plain `cask "voxtype"` in the `Brewfile` isn't
trusted, so `install.sh` stopped there on the first Mac run. **Fix:** list
third-party items fully qualified with `trusted: true` (`cask
"peteonrails/voxtype/voxtype", trusted: true`), which trusts just that cask,
not the whole tap. By hand: `brew trust --cask <user>/<tap>/<name>`.

### Personal-only apps installed on the work Mac

**Why:** an early Brewfile read `ENV["DOTS_PROFILE"]`, which Homebrew strips
(it runs with a cleaned environment), so it always saw "personal". It now
reads `~/.config/dots/profile`. **Fix:** check that file says `work`, then
list what isn't in the Brewfile with `brew bundle cleanup --file=~/Work/dots/Brewfile`
(dry run) and remove each with `brew uninstall --cask --zap <name>` /
`brew uninstall <name>`, then `brew autoremove`.

### `defaultbrowser` doesn't mark Chrome as current

**Why:** it lists `chrome` without the `*` even when Chrome is the default
(the bundle ID's case differs: `com.google.Chrome` vs `com.google.chrome`).
`install.sh` reads macOS's own setting instead:
`plutil -convert json -o - ~/Library/Preferences/com.apple.LaunchServices/com.apple.launchservices.secure.plist | jq '.LSHandlers[] | select(.LSHandlerURLScheme == "https")'`.

### btop on the Mac has different colors than on Omarchy

**Why:** Omarchy's `btop.conf` uses `color_theme = "current"`, a link to
Omarchy's current theme, which the Mac doesn't have; btop falls back to
its default theme
([macos.md](../decisions/macos.md#omarchys-config-files-on-the-mac)).
**Fix (optional):** pick a built-in theme in btop's menu (Esc > Options),
or put a `.theme` file in `~/.config/btop/themes/current.theme`.

### `install.sh` says "has local changes; skipping update" (`.bash_profile`)

**Why:** `make setup` ran where neither mise's `prek` nor Homebrew was on
PATH (seen over SSH on the MacBook, 2026-10-19), so it fell back to prek's
own installer, which appended `. "$HOME/.local/bin/env"` to `~/.bashrc`,
`~/.bash_profile` (a link into dots) and new `~/.profile`, `~/.zshrc` and
fish files. The Makefile now runs that installer with
`PREK_NO_MODIFY_PATH=1`. **Fix:** `git -C ~/Work/dots checkout --
macos/.bash_profile`, remove the line from `~/.bashrc` and the files it
created (`~/.local/bin/env`, `env.fish` and `prek`, `~/.config/prek`, plus
`~/.profile`, `~/.zshrc`, `~/.config/fish` if they hold only that line),
then re-run `install.sh`. Run `make setup` from a normal terminal.

### Mac: want OneDark back

**Why:** the Mac moved to the Moonlight theme on 2026-10-19
([macos.md](../decisions/macos.md#theme-moonlight)); OneDark "dark"
(Ghostty's `onedark-dark`, onedark.nvim) was dropped. **Fix:** in
`~/.config/ghostty/config.local`, `theme = Atom One Dark` (bundled with
Ghostty; close to it) and `background-opacity = 1`; for Neovim, restore
`macos/.config/nvim/lua/plugins/theme.dots.lua` from git history
(`git log --diff-filter=D -- macos/`) and remove the `theme.lua` link.

### Mac: the wallpaper isn't the fisherman / "Could not set the wallpaper"

**Why:** System Events sets the wallpaper of the current Space only, and
needs the terminal to be allowed to control it. **Fix:** allow it in
System Settings > Privacy & Security > Automation and re-run `install.sh`,
or pick `~/Work/omarchy-moonlight-theme/backgrounds/1-moonlit-fisherman.png`
in System Settings > Wallpaper (on each Space).

### Claude Code on a Mac has no Todoist skill

**Why:** before 2026-10-13, `install.sh` ran the Todoist skill step on
Omarchy only. **Fix:** re-run `install.sh` (the step is shared now), or
`td skill install claude-code` and `td skill install universal`.

### "Bad CPU type in executable" on a Mac

**Why:** an Intel-only program (e.g. wkhtmltopdf) and no Rosetta 2.
**Fix:** re-run `install.sh` (it installs Rosetta), or
`softwareupdate --install-rosetta --agree-to-license`.

### Chrome on the Mac still offers to save passwords

**Why:** `install.sh` sets Chrome's `PasswordManagerEnabled` policy to false
at the *recommended* level (no MDM), which a choice made in Chrome's
settings overrides. **Fix:** Chrome Settings → Password Manager → gear →
turn off **Offer to save passwords**; check `chrome://policy` lists
`PasswordManagerEnabled` (Reload policies). Turning on 1Password's "default
password manager" setting alone didn't stop it (2026-10-13).

### `try` isn't on the Mac

**Why:** dropped from the Brewfile (2026-10-13,
[macos.md](../decisions/macos.md#brewfile-section-1-command-line-tools)): unused, and it
brought Homebrew's Ruby, which non-interactive shells found before mise's.
Omarchy's bash init only defines `try` when the command exists, so nothing
else breaks. **To bring back:** add `brew "try"` to the Brewfile's
"Mirroring Omarchy's tools" list and re-run `install.sh` (Homebrew's Ruby
comes back with it; use `mise exec` or an interactive shell for project
Ruby in scripts).

### `sff` fails on a Mac ("find: -printf: unknown primary")

**Why:** Omarchy's `sff` (pick a recent file, `scp` it) uses GNU `find
-printf`; macOS's `find` doesn't have it. Not patched (rarely used;
[macos.md](../decisions/macos.md#bash-omarchys-defaults-plus-a-mac-add-on)).
**Workaround:** `scp "$(ff)" host:/tmp/`.

### Mac shell: `omarchy-…` errors, or `open` doesn't work

**Why:** `OMARCHY_PATH`, `EDITOR` and `BROWSER` come from
`~/.config/bash/macos-env`, loaded by `~/.bash_profile`, i.e. login shells
only (Terminal, Ghostty and tmux start those). A shell started without
that environment falls back to Omarchy's Linux values. **Fix:** open a new
terminal window, or `source ~/.bash_profile`; check `echo $OMARCHY_PATH`
shows `~/.local/share/omarchy` and `echo $SHELL` shows
`/opt/homebrew/bin/bash`.

### `ipconfig getifaddr en0` prints nothing

**Why:** `en0` is Wi-Fi; on a dock or Ethernet adapter the Mac uses another
interface. **Fix:** ask for the interface carrying the default route:
`ipconfig getifaddr "$(route -n get default | awk '/interface:/{print $2}')"`.

### Mac: first `nvim` start downloads plugins / Mason installs tools

**Why:** on Omarchy the package ships a prebuilt plugin cache; on the Mac
there isn't one (it's built for Linux), so the first start downloads ~60
plugins, compiles Tree-sitter parsers and has Mason install language tools.
Expected; let it finish (`:Lazy`, `:Mason` show progress). Run headless
(`nvim --headless "+Lazy! sync" +qa`), it quits before Mason is done and
prints *"Neovim exited while the following packages were installing"*:
they install on the next normal start.

### Mac: Neovim paste fails, "Waiting for OSC 52 response" / E353

**Why:** inside tmux, Omarchy's `remote_clipboard.lua` pastes by asking the
terminal (OSC 52) when there's no Wayland, and tmux answers only from its
own paste buffers, so text copied in other apps never arrives. **Fix (in
dots):** the `macos` package's `lua/config/options.dots.lua` clears that
outside SSH, so Neovim uses `pbcopy`/`pbpaste`. If it comes back, check the
include line at the end of `~/.config/nvim/lua/config/options.lua` and
`:lua print(vim.g.clipboard)` (should print `nil`); re-run `install.sh`.

### Mason: "Could not find executable "npm"" / "go" (e.g. jsonls fails)

**Why:** Mason builds some tools with npm (json-lsp, yaml-language-server,
pyright, Docker and Ansible language servers, markdownlint-cli2...) or go
(gopls, goimports, gofumpt); without Node or Go they fail
(`~/.local/state/nvim/mason.log`). Seen on the Mac before its mise stage.
**Fix:** Node and Go come from mise (`mise use -g node@latest`; `go` in
`config-dots.toml`): re-run `install.sh`, then reopen `nvim` and Mason
retries on its own (or `:MasonInstall <name>`).

### Mac: `docker compose` / `docker buildx`: "unknown command"

**Why:** Homebrew installs them as Docker CLI plugins in
`/opt/homebrew/lib/docker/cli-plugins`, which Docker only searches when
`~/.docker/config.json` lists it in `cliPluginsExtraDirs`. **Fix:**
`install.sh` adds it; re-run it (or add the key by hand).

### Mac: a container can't see a folder (`-v` mount is empty)

**Why:** Colima shares only your home folder with its VM; paths elsewhere
(`/tmp`, `/var/folders`, `mktemp -d`) don't exist inside it. **Fix:** keep
projects under `~` (e.g. `~/Work`), or add a mount: `colima stop`,
`colima start --edit` (`mounts:`).

### Mac: `Include ~/.colima/ssh_config` appears in `~/.ssh/config` (or in dots)

**Why:** Colima's `sshConfig: true` (its default) adds that line on every
start, and `~/.ssh/config` is dots' file. **Fix:** `install.sh` sets
`sshConfig: false` in `~/.colima/default/colima.yaml`; remove the line
from `ssh/.ssh/config` if it got in. `colima ssh` keeps working.

### Mac: Colima created a `qemu` VM / "qemu-img not found"

**Why:** Colima's template (`colima template`) hardcodes `vmType: qemu` and
`mountType: sshfs`, unlike the command's macOS defaults (`vz`, `virtiofs`);
a new VM created from an edited template gets qemu. **Fix:** set
`vmType: vz` and `mountType: virtiofs` in the template, then
`colima delete` and start again (the VM type can't change after creation).
`install.sh` sets both in its first-setup config.

### Mac: Tailscale "Start on Login could not be set" popup

**Symptom:** ticking **Launch Tailscale at login** in the Tailscale app
shows *"Start on Login could not be set. Enable Tailscale in the System
Settings app ('General' -> 'Login Items')"*, and Tailscale isn't listed
there. **Why:** macOS's background-items service has no record of the
app's login helper (`log show --predicate 'process ==
"backgroundtaskmanagementd"'`: *"effectiveItemDisposition: record not
found: appURL=/Applications/Tailscale.app,
url=/Contents/Library/LoginItems/TailscaleLoginItemHelper-macsys.app"*),
so it can't enable it. Seen 2026-10-11 on the work MacBook (Tailscale
1.104.1 from the `tailscale-app` cask, macOS 27.0.1; not MDM-enrolled).
Re-registering the app (`lsregister -f /Applications/Tailscale.app`, then
reopening it) and restarting didn't help. **Workaround (works):** System
Settings > General > Login Items & Extensions > **Open at Login** > **+** >
Tailscale; it starts at login (verified after a restart). `install.sh` now
adds this entry itself (2026-10-12). The checkbox stays broken. Not tried: `sfltool resetbtm` (resets every app's
background-item approvals). Follow-up in [TODO](../TODO.md).

### Mac: Keyboard sliders don't move after `defaults write -g KeyRepeat`

**Why:** System Settings shows the repeat speed that's active now, loaded
from preferences at login, and on macOS 27 it uses floats (`-float 2`, not
`-int 2`). **Fix:** `install.sh` writes floats and runs
`/System/Library/PrivateFrameworks/SystemAdministration.framework/Resources/activateSettings -u`,
which applies them at once; otherwise log out and back in. To change a
stored integer into a float, `defaults delete -g KeyRepeat` first: a write
of an equal number is ignored.

### Mac: a reverted macOS setting (Dock, Finder, typing)

`install.sh` re-applies its `MACOS_DEFAULTS` on every run, so a setting you
change by hand in System Settings is put back the next time. To keep your
own value, remove that line from `MACOS_DEFAULTS`
([macos.md](../decisions/macos.md#macos-settings-defaults)). The F-keys are
deliberately not set: hold Fn for F1-F12.

### Mac: `sha256sum -c --status` prints "usage: sha256sum [-bctwz]"

**Why:** macOS 27's own `sha256sum` (`/sbin`) has no `--status`, and its
`-c` returned 1 even for a correct line (tested 2026-10-11). **Fix:** hash
from stdin and compare, which works on both systems:
`[[ $(sha256sum <file) == "<hash> "* ]]` (as `install_tmux_ssh` does).

### Mac: beeping while holding the Voxtype hotkey

**Why:** Voxtype's macOS hotkey only listens (listen-only event tap), so
the key also reaches the app in front; an app with nothing on that key
plays the alert sound, again on every key repeat. On Omarchy, Hyprland
takes the key. **Fix:** dots leaves the hotkey off; record from the menu
bar ([macos.md](../decisions/macos.md)). Hammerspoon is the possible way
back to a key ([TODO](../TODO.md)).

### Mac: Voxtype stopped typing or hearing the hotkey after an upgrade

**Why:** `voxtype setup app-bundle` (run by `install.sh` after `brew
upgrade` changes the version) copies the new binary and resets its
Accessibility and Input Monitoring permissions. **Fix:** turn both back
on for Voxtype in System Settings > Privacy & Security, then `pkill -f
voxtype-bin; open /Applications/Voxtype.app`.

### Mac: Voxtype logs "playerctl not found" / "Failed to spawn voxtype-osd"

Harmless Linux leftovers: `pause_media` uses `playerctl` (dots seeds it
off on the Mac; an older seeded config may still have it on), and the
on-screen display is a Linux helper. The menu bar icon shows the state.

### Mac: Syncthing stops connecting after `brew upgrade` (firewall on)

**Why:** the firewall allows apps by path, and Homebrew's Syncthing lives
in a versioned folder (`/opt/homebrew/Cellar/syncthing/<version>/`), so
the upgraded binary may be a new, unallowed app. Sync still works by the
Mac dialing out to Omarchy; to accept incoming connections again, allow
it in System Settings > Network > Firewall > Options (or `sudo
/usr/libexec/ApplicationFirewall/socketfilterfw --add $(brew --prefix
syncthing)/bin/syncthing`), then `brew services restart syncthing`.

### Syncthing: get back an old or deleted note

Omarchy and the Mac Studio keep versions in the vault's `.stversions`
folder (file name plus a timestamp, e.g. `note~20261012-170153.md`), for
changes made on **another** machine: look on the machine where you didn't
make the change. Copy the version back into the vault, or use the web UI
(folder > Versions). Kept up to 365 days.

### Syncthing: a machine was wiped or reinstalled

**Why:** Syncthing's device ID comes from a key made on first start, so a
reinstalled machine is a new device. **Fix:** check Omarchy (the hub) shows
the vaults Up to Date **before** wiping; afterwards remove the old device
on Omarchy, run `install.sh` on the reinstalled machine (it recreates the
folders, `.stignore` and versioning, and on a Mac pairs with Omarchy), then
approve it on Omarchy, and it syncs back.

### Mac: Syncthing's device name is `ip-10-0-0-…compute.internal`

**Why:** Syncthing names itself after the hostname; a Mac with no HostName
gets one from reverse DNS, which the AWS VPN answers. **Fix:** rename the
device in Syncthing's web UI (Actions > Settings). `install.sh` now sets a
HostName first, so a new Mac's Syncthing starts with `mac-studio` /
`macbook-pro`.

### Mac: `:checkhealth lazy` shows a luarocks ERROR

**Why:** the Mac has no `luarocks`/Lua 5.1 (Omarchy installs both), so
lazy.nvim's *hererocks* check fails. Harmless: the same report says *"no
plugins require `luarocks`, so you can ignore any warnings below"*.

### `brew bundle` warns about the Voxtype tap (`depends_on macos:`, `postflight`)

**Why:** Homebrew 7 deprecated those forms and the `peteonrails/voxtype`
tap still uses them. Only warnings; the cask installs. Goes away when the
tap is updated.

### Two notifications for every Teams message

**Why:** Teams was running twice: the Teams web app and teams-for-linux
(tried by hand, then dropped: packages.md). **Fix:** run only the web app:
`omarchy-pkg-remove teams-for-linux-bin` (quit it from the tray first), and
close any Teams tab in Chrome as well.

### `private words` fails a commit or `make lint`

**Why:** a staged file or the commit message contains a word from
`~/.config/dots/private-words` (the public repo must not name the
employer; [secrets.md](../decisions/secrets.md#private-words-a-lint-check-not-a-habit-decided-2026-10-19)),
or the list is missing on this machine, or one of its patterns is broken.
**Fix:** reword the lines it prints (it prints them only locally). If the
list is missing, copy it from another machine (mode 600); to commit without
the check, leave the file with only comments.

## Syncthing

### Claude Code: an extra session, or a `sync-conflict` file

**Why:** the same session was open on two machines, and both wrote to it
before Syncthing caught up; Syncthing keeps both and renames one
`<id>.sync-conflict-<date>-<time>-<device>.jsonl`, which `claude --resume`
may list as an odd extra session. **Fix:** resume the real one (the name
without `sync-conflict`); if the copy has turns you want, `claude --resume`
it once and ask for what's missing, then delete it. Keep a session on one
machine at a time.

### Claude Code: a session or memory didn't reach another machine

**Why:** it's in a path-named folder (`-home-…`, `-Users-…`), which
`.stignore` keeps local: Claude was started without the `claude` wrapper
(herdr, an editor, `command claude`) or before the machine's switch-over
(`~/.claude/.claude.json`). **Fix:** start it through the wrapper; move an
existing session with `claude-migrate-projects --apply`. Also check the
folder is shared with that machine in Omarchy's Syncthing UI.

**Synced, but `claude --resume` doesn't list it:** the shell started before
the switch-over. The wrapper checks for `~/.claude/.claude.json` only when
bash starts, so a shell (or tmux pane) opened earlier runs plain Claude,
which uses `~/.claude.json` and lists the path-named folder's sessions
(found on the MacBook 2026-10-19, in a tmux pane from a week before). Check
with `type -t claude` (`function` when it's on). **Fix:** open a new shell
or tmux window, or `exec bash -l` in the old one.

### Claude Code: get back an old or deleted session

Omarchy keeps the previous copy of each session or memory file a Mac
changed or deleted, for 30 days, in
`~/.local/state/syncthing-versions/claude-projects` (same paths as in
`~/.claude/projects`, with a timestamp). Copy it back while Claude isn't
running in that project. `~/.agents` versions are in `~/.agents/.stversions`
(365 days).

### `.sync-conflict` files or a broken git repo inside a synced folder

**Why:** a git repo (or `~/Work`) was put in a Syncthing folder. Syncthing
and git both rewrite `.git`; two machines editing before a sync finishes
leaves conflict copies inside `.git`. **Fix:** keep repos out of Syncthing
(sync them with git push/pull); remove the folder from sharing, delete the
`.sync-conflict*` files, re-clone if git is confused
([packages.md](../decisions/packages.md)).

## Missing tools

### `htop` / `btm` / `ncdu` / `colordiff` / `thefuck` / `act` / `hunk` not found

**Why:** dropped in favor of what Omarchy ships: `btop` (system monitor),
`dua i` (disk usage), `diff --color`. `thefuck` and `act` were dropped. `hunk` (AI-agent diff
reviewer) was dropped for Omarchy's **lazygit**; add it with
`omarchy-pkg-aur-add hunk-bin` (AUR).
**Re-add** an official-repo package to `PACMAN_PACKAGES` in `install.sh`
(installed with `omarchy-pkg-add`).

### `colortest` not found

**Why:** the old color-check script was dropped
([bin.md](../decisions/bin.md)). Same output, one line:

```sh
for i in {1..7}; do echo " $(tput setaf $i)Text$(tput sgr0) $(tput bold)$(tput setaf $i)Text$(tput sgr0) $(tput smul)$(tput setaf $i)Text$(tput sgr0)  setaf $i"; done
```

### `backup-to-thumb-drive` / `import-from-thumb-drive` / `push-to-mac-studio` / `pull-from-mac-studio` not found

**Why:** dropped; remote desktop into this machine replaces carrying work
between computers. What they copied, the `~/.claude` exclude list and how
to port them to Omarchy: [bin.md](../decisions/bin.md).

## Shell

### `cd -2` / directory stack, `autocd`, `**` globs don't work

**Why:** zsh-only features; dropped in [shell.md](../decisions/shell.md).
Use zoxide (`cd <partial-name>`). For `**`: `shopt -s globstar`; for
typing a directory name to cd: `shopt -s autocd` (`~/.bashrc.local`).

### `[vim]` appears at the start of the prompt

**Why:** that's the vi **command-mode** indicator (readline
`show-mode-in-prompt` in `~/.config/bash/inputrc`); press `i` or `a` to go
back to insert mode. Change or remove it via `vi-cmd-mode-string` /
`show-mode-in-prompt`.

### Prompt shows git status symbols / cyan directory (not the old prompt)

**Why:** Omarchy's `starship.toml` is used as is; the old one (git status
off, blue directories, nerd-font symbols) was dropped. Starship has no
include mechanism, so customizing means a full override: stow a
`starship.toml`, or point `STARSHIP_CONFIG` at a dots file in
`~/.bashrc.dots`.

### `gcm` switches to master instead of committing

**Why:** dots' `~/.config/bash/aliases` sets `gcm` to `git switch master`
(the old Mac muscle memory), replacing Omarchy's `gcm` = `git commit -m`;
commit with a message using `gcmsg` (same command). **Restore Omarchy's:**
delete dots' `alias gcm=` line (or add `alias gcm='git commit -m'` to
`~/.bashrc.local`).

### A git alias like `gco`, `gp` is missing

**Why:** the old git aliases were dropped to learn Omarchy's (`g`,
`gcam`, `gcad`, `ga`/`gd` worktrees); a few were re-added for muscle memory
(`gs`, `ggpur`, `gftf`, `ggpurt`, `grhh`, `gcm`, `gcmsg`). Add individual
ones to the git section of `~/.config/bash/aliases`, checking Omarchy's
`default/bash/aliases` for a clash first.

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

## dots itself

### `op` can't find an item that's in 1Password (two accounts)

**Why:** with more than one account in the 1Password app, `op` without
`--account` only searches its default account. **Fix:** dots names the
account on every call (`dots_op_accounts`, `aws-role-login` searches all);
by hand: `op account list`, then `op --account <ID or sign-in address> …`.

### 1Password: `op` can't import an existing SSH key

Tested 2026-10-19 (`op` 2.39.0): `op item create` with an `SSH_KEY` template
whose `private key` (`SSHKEY`) field holds an OpenSSH private key creates an
item without a usable key (no `private key` or `public key` field to read).
`op` only generates new keys (`--ssh-generate-key`). **Fix:** import the key
file in the 1Password app (New Item > SSH Key > Add Private Key > Import a
Key File), with the title and `dots/ssh` tag `install.sh` expects; then
check `op read "op://<vault>/<item>/public key" | ssh-keygen -lf -`.

### `aws-role-login`: "need exactly one 1Password item tagged dots/aws-mfa"

**Why:** it searches every signed-in 1Password account, and found the tag
on more than one item (found 2026-10-19: a leftover copy of the work item
in the personal `Private` vault, visible once the app was signed in to both
accounts). **Fix:** `op account list`, then per account `op --account <ID>
item list --tags dots/aws-mfa`; keep the one in the right account
(compare, don't guess) and archive or untag the other. Meanwhile pass the
code: `aws-role-login "$(op --account <ID> item get '<item>' --otp)"`.
`generate-ssh-config` failing with `InvalidClientTokenId` on `AssumeRole`
just means the `mfa` session has expired: run `aws-role-login`.

### Mac over SSH: `gh` says "The token in default is invalid"

**Why:** `gh` keeps its token in the macOS Keychain, which stays locked in
SSH sessions, so `gh` can't read it there. It's fine in a local terminal
(`gh auth status` shows `(keyring)`). Don't sign in again over SSH: `gh`
would store the new token in plain text (`~/.config/gh/hosts.yml`);
`gh-role-login` leaves a Keychain token alone over SSH, and moves a
plain-text one into the Keychain when run locally.

### Work Mac: `gh` or `git push` refused for a personal repo (403)

Expected: the work Mac's token is limited to the company organization
([git.md](../decisions/git.md)). Push personal repos (dots) from a personal
machine. If the token expired: renew it on GitHub, paste it into the
1Password item tagged `dots/gh`, run `gh-role-login`.

### SSH key refused after adding it to `authorized_keys` (and the old one too)

**Why:** `echo key >> ~/.ssh/authorized_keys` on a file whose last line
has no newline glues the new key onto the previous one, breaking **both**
(seen on the Mac Studio, 2026-10-11). **Fix:** log in another way
(password, or at the machine), back the file up, and split the line:
`perl -pi -e 's/(\S)(ssh-ed25519 AAAA)/$1\n$2/g' ~/.ssh/authorized_keys`.
**Avoid:** add a newline first when the file doesn't end with one:
`[ -n "$(tail -c1 f)" ] && echo >> f`. Omarchy's
`omarchy-setup-security-sshd` appends the same way.

### `op`: "authorization prompt dismissed, please try again"

**Why:** the 1Password app asks to approve CLI access (on the screen of
the machine running `op`, also for commands run over SSH) and the prompt
timed out. **Fix:** approve it in time and re-run.

### Moving a 1Password item to another account

`op item move` only moves between vaults of one account, and `op item
create` can't import an existing SSH key (it creates an unreadable item).
Use the 1Password app (item ⋯ > Move, with both accounts signed in), then
check fingerprints or hashes against the installed files.

### `install.sh` stops at "Checking git identity" ("could not open TTY")

**Why:** `gum input` needs a terminal; run without one (`ssh host
'~/Work/dots/install.sh'`), it failed and `set -e` ended the run. **Fixed:**
without a terminal the step now sets the defaults (`DOTS_GIT_NAME`,
`DOTS_GIT_EMAIL` in `lib/dots.sh`). To use another identity on a machine:
`git config --global user.name …` / `user.email …`.

### Notification "dots needs attention" (after `omarchy update`)

**Why:** the post-update hook (`dots.hook`, same as `make doctor`) found
drift. Details are in the update output (also `/tmp/omarchy-update.log`),
or run `make doctor` in `~/Work/dots`:

- *Re-added the dots include line to …*: an Omarchy file was reset (e.g.
  `omarchy refresh`); already fixed, nothing to do.
- *… is not linked to dots*: something replaced a stowed link with a real
  file. Compare it with the repo copy, keep what you want, then run
  `install.sh` (it backs the real file up and restows).
- *… has uncommitted changes*: review `git -C ~/Work/dots status`. Either
  your own unfinished edits, or a migration wrote through a link into the
  repo (commit or `git restore` it).

### Mac: `make doctor` says every file "is not linked to dots"

**Why:** it used `realpath -m`; macOS's `realpath` has no `-m` (prints
"illegal option -- m"), so every check failed. **Fixed** (2026-10-11): plain
`realpath`, which works on both systems.

### A custom Omarchy theme loses its colors (stowed theme files)

**Symptom:** after `omarchy theme set`, a theme whose files are symlinks
(e.g. stowed file by file) gets no colors or Neovim theme.
**Cause:** `omarchy theme set` copies the theme with `cp -r` into
`~/.local/state/omarchy/current/next-theme`; relative links are copied as
links and point nowhere from there. **Fix:** link the whole theme folder
instead, as `install.sh` does for One Dark
(`~/.config/omarchy/themes/one-dark` -> `~/Work/dots/themes/one-dark`;
[desktop.md](../decisions/desktop.md#theme-one-dark-our-own-themesone-dark)).

### Moonlight (or any theme) has no terminal transparency or its own Neovim colors

**Symptom:** the theme's colors apply, but foot is solid and Neovim isn't
Catppuccin Macchiato; `omarchy theme set` printed *"Ignored in ...:
foot.ini neovim.lua"*. **Cause:** the theme folder is a real git clone
(from `omarchy theme install`), which Omarchy treats as a stranger's theme
and strips of files that can run programs. **Fix:** link it to a working
copy instead, as `install.sh` does:
`rm -rf ~/.config/omarchy/themes/moonlight`, re-run `install.sh` (or
`ln -s ~/Work/omarchy-moonlight-theme ~/.config/omarchy/themes/moonlight`),
then `omarchy theme set Moonlight` and open a new terminal
([desktop.md](../decisions/desktop.md#theme-moonlight-the-mac-studios-catppuccin-look-its-own-repo)).

### A band around the inside edge of the terminal

**Symptom:** a ~14 px frame inside foot's border (its padding, `pad=14x14`)
looks more transparent, or a slightly different color, than the text area.
**Cause:** on every theme switch Omarchy's `omarchy-theme-set-tmux` sets
tmux's `window-style` to the theme's background, so tmux paints each cell
in that exact color, while foot draws the padding itself. (1) With foot
`alpha` below 1, foot keeps explicitly colored cells solid (`alpha-mode`
default), so only the padding is see-through. (2) foot can't reload its
colors, so after a theme switch the padding keeps the color of the theme
active when the window opened. **Fix:** (1) `alpha-mode=matching` next to
`alpha` (Moonlight's `foot.ini` has it); (2) reopen the terminal
after switching themes.

### tmux bar doesn't change after switching themes

**Symptom:** after `omarchy theme set`, tmux still shows the previous
theme's bar (e.g. Moonlight's Macchiato bar under One Dark).
**Cause:** tmux reads its config only when loaded; dots' theme-set hook
(`~/.config/omarchy/hooks/theme-set.d/dots-tmux.hook`) reloads it on every
switch. **Fix:** check the hook is linked (`make doctor`), or reload by
hand with `prefix q`. A theme's bar lives in
`~/.config/tmux/themes/<slug>.conf`, named after the theme's folder
(`cat ~/.local/state/omarchy/current/theme.name`); a renamed theme needs its
bar file renamed too.

### `make lint` changes a word you wrote (e.g. the plural of PNG)

**Symptom:** the typos hook fails and the word in the file has changed.
**Cause:** the hook runs with `--write-changes`, so it applies its own
correction; the plural of PNG is read as a misspelling and rewritten to
"ONGs". Adding the word to `.typos.toml` didn't stop it (tried
2026-10-17). **Fix:** check `git diff` after a typos failure; reword (e.g. "PNG files"), or for an ID add it under
`[default.extend-identifiers]` in `.typos.toml`.

### Claude Code: first-run setup again, or settings gone (`CLAUDE_CONFIG_DIR`)

Setting `CLAUDE_CONFIG_DIR` (even to `~/.claude`) makes Claude Code use
`$CLAUDE_CONFIG_DIR/.claude.json` instead of `~/.claude.json` (account,
trusted folders, MCP servers). Found testing 2026-10-18 (v2.1.288): with no
file there it restored an old backup from `~/.claude/backups/` and showed the
first-run theme picker. **Fix:** before setting it, copy the current
`~/.claude.json` there, then always launch with it set (two copies drift).
dots' `claude` wrapper (`~/.config/bash/claude`) sets it only once
`~/.claude/.claude.json` exists, so that copy is the switch.

### Claude Code: migration says "left alone: session ... differs"

`claude-migrate-projects --apply` found a copy of that session in the new
project folder that isn't the original or an earlier part of it (it was
continued in both places, or edited). It never overwrites one. **Fix:**
keep the copy you want: compare the two `<id>.jsonl` files (old folder vs
`~/.claude/projects/<name>/`); to take the old one, copy it over, or resume
the other with `claude --fork-session --resume <id>`.

### Claude Code: a session or memory is missing on this machine

dots' `claude` wrapper (interactive bash only) names each project folder
after the repo (`claude-project-name` in a folder prints it), so every
machine uses one folder. Claude started another way (a script, an editor,
`/usr/bin/env claude`, another shell) or in a folder whose override is
`-` uses Claude's own path-named folder (`-home-...`), which isn't synced.
**Fix:** start it from bash with `claude`/`cx`; find where a session went
with `ls -d ~/.claude/projects/*/<session id>.jsonl`; `claude --resume
<id>` finds it in any project folder.

### Claude Code: memory "needs permission" in a symlinked project folder

If `~/.claude/projects/<folder>` is a symlink to another folder, Claude reads
the `MEMORY.md` index but asks permission to read memory files and refuses to
save new ones, even with `Read`/`Edit`/`Write(~/.claude/projects/**)` allow
rules (tested 2026-10-18, v2.1.288). Sessions work through the link. **Fix:**
don't symlink project folders; name the folder with
`CLAUDE_CODE_PROJECT_DIR_NAME` (needs `CLAUDE_CONFIG_DIR` set) instead.

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
ships the fix *and* updates your copy. The Mac's `install.sh` pins the
fixed commit (`OMARCHY_NVIM_REF`), so a Mac never has this.

### Lazy shows "Breaking Changes" / "updates available"

Not an error: lazy.nvim lists newer versions than the ones pinned in
Omarchy's `lazy-lock.json`. Updating (`:Lazy update`) is optional and
moves you off Omarchy's pinned versions.
