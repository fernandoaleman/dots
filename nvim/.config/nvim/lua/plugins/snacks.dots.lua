-- Search hidden (dot) files by default in the file and grep pickers;
-- git-ignored files stay skipped (toggle: <A-h> / <A-i>). The .git
-- directory is never searched: both sources always exclude it. (The file
-- explorer is Omarchy's neo-tree: see neo-tree.dots.lua.)
return {
  "folke/snacks.nvim",
  opts = {
    picker = {
      sources = {
        files = { hidden = true },
        grep = { hidden = true },
      },
    },
  },
}
