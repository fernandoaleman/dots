-- macOS only: the Mac has no Omarchy theme switcher, so no theme.lua link
-- into the current Omarchy theme. OneDark "dark", the palette Ghostty's
-- onedark-dark theme is built from; same shape as Omarchy's theme specs
-- (docs/decisions/macos.md).
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
