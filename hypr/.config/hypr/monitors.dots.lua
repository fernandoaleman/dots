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

-- Desk: three 32" curved LG UltraGear 1440p monitors, left to right.
-- The right one is on HDMI, which tops out at 144Hz.
local left = "desc:LG Electronics LG ULTRAGEAR 408BOPY0P208"
local middle = "desc:LG Electronics LG ULTRAGEAR 407BOYQ0E522"
local right = "desc:LG Electronics LG ULTRAGEAR 407BOPY0E520"

hl.monitor({ output = left, mode = "2560x1440@180", position = "0x0", scale = 1.6 })
hl.monitor({ output = middle, mode = "2560x1440@180", position = "1600x0", scale = 1.6 })
hl.monitor({ output = right, mode = "2560x1440@144", position = "3200x0", scale = 1.6 })

-- Workspaces 1-3 on the left, 4-6 in the middle, 7-9 on the right, so
-- Super+N always lands on the same physical monitor. The first of each
-- group (1, 4, 7) is the monitor's default workspace. Workspaces bound to
-- a disconnected monitor open on the current one instead.
for i = 1, 9 do
  local monitor = (i <= 3 and left) or (i <= 6 and middle) or right
  hl.workspace_rule({ workspace = tostring(i), monitor = monitor, default = (i % 3 == 1) })
end

-- Machine-local overrides, not stored in dots (skipped if missing)
do
  local f = os.getenv("HOME") .. "/.config/hypr/monitors.local.lua"
  local h = io.open(f)
  if h then h:close() dofile(f) end
end
