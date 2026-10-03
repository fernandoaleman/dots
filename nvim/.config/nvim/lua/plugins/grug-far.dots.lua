-- Inside a grug-far buffer: <A-h> toggles hidden files, <A-i> toggles
-- git-ignored files (LazyVim's <leader>sr respects both by default).
-- Uses `init`, not `config`, so LazyVim's grug-far setup is left intact.
return {
  "MagicDuck/grug-far.nvim",
  init = function()
    vim.api.nvim_create_autocmd("FileType", {
      pattern = "grug-far",
      callback = function(ev)
        local function toggle(flags, label)
          local state = unpack(require("grug-far").get_instance(0):toggle_flags(flags))
          vim.notify("grug-far: " .. label .. " " .. (state and "ON" or "OFF"))
        end
        vim.keymap.set({ "i", "n", "x" }, "<A-h>", function()
          toggle({ "--hidden", "--glob !.git/" }, "hidden files")
        end, { desc = "Toggle Hidden Files", buffer = ev.buf })
        vim.keymap.set({ "i", "n", "x" }, "<A-i>", function()
          toggle({ "--no-ignore" }, "ignored files")
        end, { desc = "Toggle Ignored Files", buffer = ev.buf })
      end,
    })
  end,
}
