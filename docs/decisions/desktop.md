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
- Desk: three 32" curved LG UltraGear 1440p monitors, left to right; only
  the left one (`407BOPY0E520`, 144Hz, on HDMI) is connected so far. Scale
  1.6 (Omarchy's), so positions are in scaled pixels (next monitor at
  x = 1600). Workspace 1 is bound to the left monitor; 2 and 3 get bound
  when the other two are connected (TODO in the file).
- Tested live (2026-10-03): no `hyprctl configerrors`; a workspace bound to
  a **disconnected** monitor simply opens on the connected one; the
  `monitors.local.lua` layer loads. (Under Lua config, `hyprctl dispatch`
  takes Lua: `hyprctl dispatch 'hl.dsp.focus({ workspace = "2" })'`.)
- Old `monitors.conf` (hyprlang, port names, scale 1, `GDK_SCALE=1`):
  replaced.

## Mac notes

Monitor arrangement and idle/lock are macOS System Settings; nothing here
carries over.
