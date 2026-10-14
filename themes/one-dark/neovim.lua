-- OneDark "dark", as on the Mac (macos/.config/nvim/lua/plugins/theme.dots.lua);
-- same shape as Omarchy's stock themes' neovim.lua.
return {
  {
    "navarasu/onedark.nvim",
    priority = 1000,
    opts = { style = "dark" },
  },
  {
    "LazyVim/LazyVim",
    opts = {
      colorscheme = "onedark",
    },
  },
}
