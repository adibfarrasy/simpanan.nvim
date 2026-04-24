local M = {}

local debounce_ms = 80

-- Map the Go SuggestionKind strings to nvim-cmp's lsp kind names.
local kind_map = {
	connection_label = "Module",
	sql_keyword = "Keyword",
	database = "Module",
	table = "Class",
	column = "Field",
	mongo_collection = "Class",
	mongo_operation = "Function",
	mongo_operator = "Operator",
	mongo_field = "Field",
	redis_command = "Function",
	jq_operator = "Function",
	jq_path = "Variable",
}

function M.new()
	local self = setmetatable({ _timer = nil }, { __index = M })
	return self
end

function M:is_available()
	-- ftdetect/simpanan.vim sets the filetype to `simpanan` for *.simp
	-- files; matching it here is what gates the cmp source.
	return vim.bo.filetype == "simpanan"
end

function M:get_trigger_characters()
	-- '|' triggers connection-label suggestions at the start of a new
	-- stage. The others are existing context-dependent triggers.
	return { "|", ".", "{", ">", "$" }
end

function M:get_keyword_pattern()
	return [[\k\+]]
end

local function cursor_byte_offset()
	local bufnr = vim.api.nvim_get_current_buf()
	local row, col = unpack(vim.api.nvim_win_get_cursor(0)) -- row: 1-indexed, col: 0-indexed bytes
	local line_start = vim.api.nvim_buf_get_offset(bufnr, row - 1)
	return bufnr, line_start + col
end

function M:_cancel_timer()
	if self._timer then
		self._timer:stop()
		if not self._timer:is_closing() then
			self._timer:close()
		end
		self._timer = nil
	end
end

function M:complete(_, callback)
	self:_cancel_timer()
	local bufnr, cursor_pos = cursor_byte_offset()

	self._timer = vim.loop.new_timer()
	self._timer:start(
		debounce_ms,
		0,
		vim.schedule_wrap(function()
			self:_cancel_timer()
			if not vim.api.nvim_buf_is_valid(bufnr) then
				callback({ items = {}, isIncomplete = false })
				return
			end
			local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
			local buffer_text = table.concat(lines, "\n")

			local ok, res = pcall(vim.fn["SimpananSuggest"], buffer_text, tostring(cursor_pos))
			if not ok then
				vim.notify("[simpanan] SimpananSuggest RPC failed: " .. tostring(res), vim.log.levels.DEBUG)
				callback({ items = {}, isIncomplete = false })
				return
			end

			local decoded_ok, decoded = pcall(vim.json.decode, res)
			if not decoded_ok or type(decoded) ~= "table" then
				callback({ items = {}, isIncomplete = false })
				return
			end

			local cmp_ok, cmp_types = pcall(require, "cmp.types.lsp")
			if not cmp_ok then
				callback({ items = {}, isIncomplete = false })
				return
			end

			local items = {}
			for _, s in ipairs(decoded) do
				local kind_name = kind_map[s.kind] or "Text"
				table.insert(items, {
					label = s.text,
					kind = cmp_types.CompletionItemKind[kind_name],
					insertText = s.text,
				})
			end
			callback({ items = items, isIncomplete = false })
		end)
	)
end

-- register wires this source into nvim-cmp. Safe to call when cmp is
-- not installed — returns false silently.
function M.register()
	local ok, cmp = pcall(require, "cmp")
	if not ok then
		return false
	end
	cmp.register_source("simpanan", M.new())
	return true
end

return M
