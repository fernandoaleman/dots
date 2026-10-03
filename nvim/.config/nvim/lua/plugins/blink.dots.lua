-- Completion keys (muscle memory): blink's super-tab preset (Tab accepts),
-- plus Enter also accepts and <C-j>/<C-k> move through the menu.
return {
  {
    "saghen/blink.cmp",
    opts = {
      keymap = {
        preset = "super-tab",
        ["<CR>"] = { "accept", "fallback" },
        ["<C-j>"] = { "select_next", "fallback" },
        ["<C-k>"] = { "select_prev", "fallback" },
      },
    },
  },
  -- Free insert-mode <C-k> (LazyVim maps it to LSP signature help)
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        ["*"] = {
          keys = {
            { "<c-k>", false, mode = "i" },
          },
        },
      },
    },
  },
}
