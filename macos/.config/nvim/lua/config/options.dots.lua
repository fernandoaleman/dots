-- macOS only, loaded last by Omarchy's lua/config/options.lua (INCLUDES in
-- lib/dots.sh). Omarchy's remote_clipboard.lua pastes from Wayland locally
-- and asks the terminal (OSC 52) only without one; the Mac has no Wayland,
-- so inside tmux every paste asked tmux, which answers only from its own
-- paste buffers (E353 for text copied in other apps). Outside SSH, drop it:
-- Neovim's own provider then uses pbcopy/pbpaste (docs/decisions/macos.md).
if not (vim.env.SSH_TTY or vim.env.SSH_CONNECTION) then
  vim.g.clipboard = nil
end

do
  local f = vim.fn.stdpath("config") .. "/lua/config/options.local.lua"
  if vim.uv.fs_stat(f) then
    dofile(f)
  end
end
