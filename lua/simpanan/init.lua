local M = {}

M.conn = {}
M.conn_count = 0
M.execute_bufnr = nil

local event = require("nui.utils.autocmd").event
local util = require("simpanan.util")

local function flush()
	M.conn = {}
	M.conn_count = 0
end

local function get_connections()
	local res, err = vim.fn["SimpananGetConnections"]()
	if err ~= nil then
		util.print_red(err)
		return
	end

	for _, r in ipairs(res) do
		table.insert(M.conn, r)
		M.conn_count = M.conn_count + 1
	end
end

function M.list_connections()
	local Popup = require("nui.popup")

	if M.conn_count == 0 then
		get_connections()
	end

	local popup = Popup({
		enter = true,
		focusable = true,
		border = {
			style = "rounded",
			text = {
				top = "[ Manage Connections ]",
				top_align = "left",
			},
			padding = {
				top = 2,
				bottom = 2,
				left = 3,
				right = 3,
			},
		},
		position = "50%",
		size = {
			width = "50%",
			height = 10,
		},
		buf_options = {
			modifiable = false,
			readonly = true,
		},
	})

	local data = {}
	for _, v in pairs(M.conn) do
		table.insert(data, v)
	end

	local menu = data
	table.insert(menu, "")
	table.insert(menu, "Hints:")
	table.insert(menu, "a → [a]dd connection      d → [d]elete connection      q → exit")
	vim.api.nvim_buf_set_lines(popup.bufnr, 0, M.conn_count, false, menu)

	popup:mount()

	popup:on(event.BufLeave, function()
		popup:unmount()
	end)

	popup:map("n", "q", function()
		popup:unmount()
	end, {})

	-- TODO: add some nice CRUD
	flush()
end

function M.execute()
	local lines = require("simpanan.util").get_visual_selection_text()
	local req = ""
	if lines ~= nil then
		for _, l in ipairs(lines) do
			req = req .. "::" .. l
		end
	end

	local res, err = vim.fn["SimpananRunQuery"](req)
	if err ~= nil then
		util.print_red(err)
		return
	end

	if M.execute_bufnr == nil or (M.execute_bufnr ~= nil and not vim.api.nvim_buf_is_loaded(M.execute_bufnr)) then
		local Split = require("nui.split")

		local split = Split({
			relative = "editor",
			position = "right",
			size = "50%",
		})

		split:mount()

		vim.api.nvim_buf_set_option(split.bufnr, "filetype", "json")

		M.execute_bufnr = split.bufnr

		split:on(event.BufWinLeave, function()
			split:unmount()
			M.execute_bufnr = nil
		end)
	end

	local cur_line = 0
	for line in string.gmatch(res, "[^\n]+") do
		vim.api.nvim_buf_set_lines(M.execute_bufnr, cur_line, -1, false, { line })
		cur_line = cur_line + 1
	end

	vim.api.nvim_feedkeys(vim.api.nvim_replace_termcodes("<Esc>", true, false, true), "n", true)
end

return M
