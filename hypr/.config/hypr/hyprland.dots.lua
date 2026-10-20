-- Loaded by the last line of Omarchy's ~/.config/hypr/hyprland.lua (added by
-- dots' install.sh), after Omarchy's defaults and bindings.

-- Super+Alt+Return: Omarchy's tmux terminal, unchanged except for an app-id
-- (xdg-terminal-exec --app-id), so the window rule below can tell it apart
-- from other terminal windows. Was: the same command without --app-id.
hl.unbind("SUPER + ALT + RETURN")
o.bind("SUPER + ALT + RETURN", "Tmux", 'omarchy-launch-terminal --app-id=tmux bash -c "tmux attach || tmux new -s Work"')

-- Omarchy tags terminals by class (default/hypr/apps/terminals.lua) and
-- Super+C/V send terminal copy/paste keys only to tagged windows; without
-- the tag the tmux window gets Ctrl+C (interrupt) and Ctrl+V (^V in Neovim).
o.window({ class = "^tmux$" }, { tag = "+terminal" })

-- Apps always open on their workspace (see monitors.dots.lua: 1-3 left,
-- 4-6 middle, 7-9 right). Matched once, when the window opens; after that
-- a window can be moved anywhere (Super+Shift+N) and stays there.
o.window({ initial_class = "^google-chrome$" }, { workspace = "1" })
o.window({ initial_class = "^tmux$" }, { workspace = "4" })
o.window({ initial_class = "^slack$" }, { workspace = "7" })
o.window({ initial_class = "^chrome-teams\\.cloud\\.microsoft__-Default$" }, { workspace = "7" })
o.window({ initial_class = "^chrome-outlook\\.office\\.com__-Default$" }, { workspace = "8" })
o.window({ initial_class = "^Spotify$" }, { workspace = "9" })

-- AWS VPN plugin panel (instead of the plugin setup writing to Omarchy's
-- bindings.lua); Super+Shift+V was free
o.bind("SUPER + SHIFT + V", "AWS VPN", "omarchy-shell fernandoaleman.aws-vpn-client toggle")

-- Raindrop Bookmarks picker (the plugin's README key); Super+Shift+R was free
o.bind("SUPER + SHIFT + R", "Raindrop bookmarks", "omarchy-shell shell toggle io.github.treramey.raindrop-bookmarks '{}'")

-- CapsLock is Hyper (its own modifier, Mod3, so no clash with Super) for the
-- CapsLock+letter keys shared with the Mac; Compose moves to Right Alt.
-- Omarchy's default is compose:caps (default/hypr/input.lua); Caps Lock
-- itself stays on both Shifts.
hl.config({ input = { kb_options = "caps:hyper,compose:ralt,shift:both_capslock_cancel" } })
o.bind("MOD3 + H", "Clipboard manager", "omarchy-shell shell toggle omarchy.clipboard")
o.bind("MOD3 + K", "Emojis", "omarchy-shell shell toggle omarchy.emojis")
o.bind("MOD3 + L", "Raindrop bookmarks", "omarchy-shell shell toggle io.github.treramey.raindrop-bookmarks '{}'")

-- Machine-local overrides, not stored in dots (skipped if missing)
do
  local f = os.getenv("HOME") .. "/.config/hypr/hyprland.local.lua"
  local h = io.open(f)
  if h then h:close() dofile(f) end
end
