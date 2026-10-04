-- Show hidden (dot) files by default in the explorer and the file/grep
-- pickers; git-ignored files stay hidden (toggle: H / I in the explorer,
-- <A-h> / <A-i> in pickers). The .git directory is never shown: the files
-- and grep sources always exclude it; the explorer needs it excluded here
-- ("/.git", so names like foo.git still show).
return {
  "folke/snacks.nvim",
  opts = {
    picker = {
      sources = {
        explorer = { hidden = true, exclude = { "/.git" } },
        files = { hidden = true },
        grep = { hidden = true },
      },
    },
  },
}
