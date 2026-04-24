local m = require("simpanan.cmp")
m.complete = function(self, _, callback)
	local bufnr = vim.api.nvim_get_current_buf()
	local row, col = unpack(vim.api.nvim_win_get_cursor(0))
	local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
	local buffer_text = table.concat(lines, "\n")
	local line_start = vim.api.nvim_buf_get_offset(bufnr, row - 1)
	local cursor_pos = line_start + col
	vim.notify(string.format("SUGGEST args: cursor=%d  buf=%s", cursor_pos, vim.inspect(buffer_text)))
	local ok, raw = pcall(vim.fn["SimpananSuggest"], buffer_text, tostring(cursor_pos))
	vim.notify(string.format("SUGGEST resp: ok=%s  raw=%s", tostring(ok), tostring(raw)))
	callback({ items = {}, isIncomplete = false })
end
