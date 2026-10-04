-- Omarchy enables LazyVim's neo-tree Extra, so <leader>e is neo-tree.
-- Show hidden (dot) files by default; git-ignored files stay hidden.
-- H toggles dotfiles and I toggles git-ignored files, like <A-h> / <A-i>
-- in the pickers (neo-tree's own H shows every filtered item at once, so
-- it couldn't hide dotfiles once they're shown by default). .git is never
-- shown.
local function toggle(option, label)
  return {
    function(state)
      local f = state.filtered_items
      f[option] = not f[option]
      require("neo-tree.sources.filesystem.commands").refresh(state)
      vim.notify("neo-tree: " .. label .. " " .. (f[option] and "OFF" or "ON"))
    end,
    desc = "Toggle " .. label,
  }
end

return {
  "nvim-neo-tree/neo-tree.nvim",
  opts = {
    filesystem = {
      filtered_items = {
        hide_dotfiles = false,
        hide_gitignored = true,
        never_show = { ".git" },
      },
      window = {
        mappings = {
          ["H"] = toggle("hide_dotfiles", "hidden files"),
          ["I"] = toggle("hide_gitignored", "ignored files"),
        },
      },
    },
  },
}
