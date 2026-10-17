-- Catppuccin Macchiato, as on the Mac Studio (its
-- ~/.config/nvim/lua/plugins/colorscheme.lua); same shape as Omarchy's stock
-- themes' neovim.lua.
return {
  {
    "catppuccin/nvim",
    name = "catppuccin",
    priority = 1000,
    opts = {
      flavour = "macchiato",
      transparent_background = true,
      styles = {
        comments = { "italic" },
      },
    },
  },
  {
    "LazyVim/LazyVim",
    opts = {
      -- The flavour's own name: Neovim 0.12 bundles a Mocha-only
      -- colors/catppuccin.vim that shadows the plugin's "catppuccin".
      colorscheme = "catppuccin-macchiato",
    },
  },
}
