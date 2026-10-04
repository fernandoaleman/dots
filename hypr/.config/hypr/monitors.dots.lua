-- Loaded by the last line of Omarchy's ~/.config/hypr/monitors.lua (added by
-- dots' install.sh), after Omarchy's rules.
--
-- Desk monitors are matched by description (model + serial), so these rules
-- only ever apply to these exact monitors. Any other screen (a laptop panel,
-- a projector) falls back to Omarchy's catch-all rule in monitors.lua.
-- Find descriptions with: hyprctl monitors all
--
-- Positions are in scaled pixels: at scale 1.6 a 2560-wide monitor is 1600
-- wide, so the next monitor to the right starts at x = 1600.

local left = "desc:LG Electronics LG ULTRAGEAR 407BOPY0E520"
-- TODO: add the middle and right monitors (old setup: 180Hz, on DP ports)
-- once connected, e.g.:
-- local middle = "desc:LG Electronics LG ULTRAGEAR <serial>"
-- hl.monitor({ output = middle, mode = "2560x1440@180", position = "1600x0", scale = 1.6 })

hl.monitor({ output = left, mode = "2560x1440@144", position = "0x0", scale = 1.6 })

-- Workspaces 1, 2, 3 open left to right
hl.workspace_rule({ workspace = "1", monitor = left, default = true })

-- Machine-local overrides, not stored in dots (skipped if missing)
do
  local f = os.getenv("HOME") .. "/.config/hypr/monitors.local.lua"
  local h = io.open(f)
  if h then h:close() dofile(f) end
end
