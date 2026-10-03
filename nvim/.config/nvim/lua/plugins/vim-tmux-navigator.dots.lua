-- Ctrl+h/j/k/l move between nvim splits and tmux panes (the tmux half is in
-- ~/.config/tmux/tmux.dots.conf). Replaces LazyVim's Ctrl+h/j/k/l window keys
-- (LazyVim skips its own maps when a plugin defines the same keys).
return {
  "christoomey/vim-tmux-navigator",
  cmd = {
    "TmuxNavigateLeft",
    "TmuxNavigateDown",
    "TmuxNavigateUp",
    "TmuxNavigateRight",
    "TmuxNavigatePrevious",
  },
  keys = {
    { "<c-h>", "<cmd><C-U>TmuxNavigateLeft<cr>", desc = "Navigate Left (nvim/tmux)" },
    { "<c-j>", "<cmd><C-U>TmuxNavigateDown<cr>", desc = "Navigate Down (nvim/tmux)" },
    { "<c-k>", "<cmd><C-U>TmuxNavigateUp<cr>", desc = "Navigate Up (nvim/tmux)" },
    { "<c-l>", "<cmd><C-U>TmuxNavigateRight<cr>", desc = "Navigate Right (nvim/tmux)" },
    { "<c-\\>", "<cmd><C-U>TmuxNavigatePrevious<cr>", desc = "Navigate Previous (nvim/tmux)" },
  },
}
