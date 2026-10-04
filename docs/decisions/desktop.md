# Desktop

Started 2026-10-03 (in progress). Old source: `dotfiles/dot_config/hypr/`
(`hypridle.conf`, `monitors.conf`), plus desktop-related chezmoi scripts.

Omarchy 4 configures Hyprland in **Lua** (`~/.config/hypr/*.lua`, loaded by
`hyprland.lua` with `require("hypr.<name>")`) and handles idle/lock in its
own shell (`~/.config/omarchy/shell.json`); `hypridle` is no longer used
(not installed).

## Idle and lock: Omarchy's

| | Old `hypridle.conf` | Omarchy (kept) |
|---|---|---|
| Screensaver | 5 min | 2.5 min (`idle.screensaver: 150`) |
| Lock | 10 min | 5 min (`idle.lock: 300`) |
| Screen off | 15 min | ~5 s after the lock screen appears while idle (the lock's blank timer runs `omarchy-brightness-display off`) |

Lock before sleep and wake handling are built into Omarchy's lock service.
To change the timings: `idle.screensaver` / `idle.lock` in `shell.json`
(JSON, rewritten by `omarchy bar …`, so it would be a `jq` merge in
`install.sh`, not a stowed file).

## Monitors: `hypr` package, `monitors.dots.lua`

- `~/.config/hypr/monitors.lua` stays Omarchy's (catch-all rule: preferred
  mode, auto position, scale 1.6, `GDK_SCALE=2`). `install.sh` appends a
  line that loads `~/.config/hypr/monitors.dots.lua` **by path** (`dofile`,
  after checking the file exists; Lua `require` can't load a dotted name).
  `dots_include` writes the marker comment with `--` for Lua files.
- Monitors are matched by **description** (`desc:` + model + serial, per the
  Hyprland wiki's *Output selection*), not by port. The rules then only
  apply to these exact monitors; any other screen (e.g. a laptop panel)
  falls back to Omarchy's catch-all (*"a fallback rule to use when no other
  rules match"*). Port names like `HDMI-A-1` aren't unique across machines.
- Desk: three 32" curved LG UltraGear 1440p monitors, scale 1.6 (Omarchy's),
  positions in scaled pixels (2560 / 1.6 = 1600 per monitor):

  | Position | Port | Description serial | Mode | Workspaces |
  |---|---|---|---|---|
  | left | `DP-1` | `408BOPY0P208` | 2560x1440@180 | 1, 2, 3 (default 1) |
  | middle | `DP-2` | `407BOYQ0E522` | 2560x1440@180 | 4, 5, 6 (default 4) |
  | right | `HDMI-A-1` | `407BOPY0E520` | 2560x1440@144 (HDMI max) | 7, 8, 9 (default 7) |

  So `Super+N` always lands on the same physical monitor. (The old config
  only bound 1–3, so 4+ opened wherever focus was.) Hyprland can't know the
  physical order; it was identified by which workspace each bar showed.
- Existing workspaces stay where they were created; to re-home one without
  logging out, focus it and run
  `hyprctl dispatch 'hl.dsp.workspace.move({ monitor = "DP-1" })'` (Omarchy
  binds the same as Super+Shift+Alt+Arrow).
- Tested live (2026-10-03): no `hyprctl configerrors`; new workspaces open
  on their bound monitor from anywhere (Super+6 → middle, Super+8 → right);
  a workspace bound to a **disconnected** monitor simply opens on the
  connected one; the
  `monitors.local.lua` layer loads. (Under Lua config, `hyprctl dispatch`
  takes Lua: `hyprctl dispatch 'hl.dsp.focus({ workspace = "2" })'`.)
- Old `monitors.conf` (hyprlang, port names, scale 1, `GDK_SCALE=1`):
  replaced.

## Mac notes

Monitor arrangement and idle/lock are macOS System Settings; nothing here
carries over.
