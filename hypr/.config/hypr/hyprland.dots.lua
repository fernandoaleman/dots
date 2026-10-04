-- Loaded by the last line of Omarchy's ~/.config/hypr/hyprland.lua (added by
-- dots' install.sh), after Omarchy's defaults and bindings.

-- Super+Alt+Return: Omarchy's tmux terminal, unchanged except for an app-id
-- (xdg-terminal-exec --app-id), so the window rule below can tell it apart
-- from other terminal windows. Was: the same command without --app-id.
hl.unbind("SUPER + ALT + RETURN")
o.bind("SUPER + ALT + RETURN", "Tmux", 'omarchy-launch-terminal --app-id=tmux bash -c "tmux attach || tmux new -s Work"')

-- Apps always open on their workspace (see monitors.dots.lua: 1-3 left,
-- 4-6 middle, 7-9 right). Matched once, when the window opens; after that
-- a window can be moved anywhere (Super+Shift+N) and stays there.
o.window({ initial_class = "^google-chrome$" }, { workspace = "1" })
o.window({ initial_class = "^tmux$" }, { workspace = "4" })
o.window({ initial_class = "^chrome-1000bulbs\\.slack\\.com__-Default$" }, { workspace = "7" })
o.window({ initial_class = "^chrome-teams\\.cloud\\.microsoft__-Default$" }, { workspace = "7" })
o.window({ initial_class = "^chrome-outlook\\.office\\.com__-Default$" }, { workspace = "8" })
o.window({ initial_class = "^Spotify$" }, { workspace = "9" })

-- Machine-local overrides, not stored in dots (skipped if missing)
do
  local f = os.getenv("HOME") .. "/.config/hypr/hyprland.local.lua"
  local h = io.open(f)
  if h then h:close() dofile(f) end
end
