-- Simpanan filetype plugin: wires up SQL syntax injection on buffers
-- whose filetype is `simpanan` (*.simp files).
--
-- The heavy lifting lives in lua/simpanan/syntax.lua so the add/delete
-- popup handlers can reuse it and refresh already-open buffers without
-- requiring the user to :edit.

local syntax = require("simpanan.syntax")

-- Defer to the next event-loop tick. The ftplugin runs on FileType,
-- before `syntax on`'s own autocmd has sourced syntax/simpanan.vim;
-- in that window `syntax include` pulls in nothing (the target
-- syntax engine is not yet primed for the buffer). Deferring makes
-- the include run after the syntax system is ready.
local bufnr = vim.api.nvim_get_current_buf()
vim.schedule(function()
  syntax.setup_sql_regions(bufnr)
end)

-- Re-run on BufEnter so returning to this buffer picks up any registry
-- changes that happened in another buffer while we were away.
local augroup = vim.api.nvim_create_augroup("SimpananSyntaxRefresh", { clear = false })
vim.api.nvim_create_autocmd("BufEnter", {
  group = augroup,
  buffer = vim.api.nvim_get_current_buf(),
  callback = function(args)
    syntax.setup_sql_regions(args.buf)
  end,
})
