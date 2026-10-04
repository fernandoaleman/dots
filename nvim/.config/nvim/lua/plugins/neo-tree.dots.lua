-- Omarchy enables LazyVim's neo-tree Extra, so <leader>e is neo-tree.
-- Show hidden (dot) files by default; git-ignored files stay hidden (H
-- shows everything filtered). .git is never shown, even with H.
return {
  "nvim-neo-tree/neo-tree.nvim",
  opts = {
    filesystem = {
      filtered_items = {
        hide_dotfiles = false,
        hide_gitignored = true,
        never_show = { ".git" },
      },
    },
  },
}
