local Popup = require("nui.popup")
local Layout = require("nui.layout")
local Input = require("nui.input")

local M = {}

local default_opts = {
	max_row_limit = 20,
	debug_mode = false,
}

M.conn = {}
M.opts = {}
M.conn_count = 0
M.execute_bufnr = nil

local event = require("nui.utils.autocmd").event
local util = require("simpanan.util")

function AddPopupHooks(popup, layout)
	popup:on(event.BufLeave, function()
		layout:unmount()
	end)

	popup:map("n", "q", function()
		layout:unmount()
	end, { noremap = true })

	popup:map("n", "a", function()
		ShowAddConnectionPopup(layout)
	end, { noremap = true })

	popup:map("n", "d", function()
		ShowDeleteConnectionPopup(layout)
	end, { noremap = true })
end

local function newConnectionPopup()
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

	return popup
end

local function get_connections()
	M.conn = {}
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

function ShowAddConnectionPopup(layout)
	layout:hide()
	local popup_options = {
		relative = "editor",
		position = "50%",
		size = {
			width = "50%",
			height = 1,
		},
		border = {
			style = "rounded",
			text = {
				top = "[ Add Connection ]",
				top_align = "left",
			},
		},
	}

	local input = Input(popup_options, {
		on_submit = function(value)
			if #value > 0 then
				local _, err = vim.fn["SimpananAddConnection"](value)
				if err ~= nil then
					util.print_red(err)
					return
				else
					get_connections()

					local popup = newConnectionPopup()
					AddPopupHooks(popup, layout)

					layout:update(Layout.Box({
						Layout.Box(popup, { size = "100%" }),
					}))
				end
			end

			layout:show()
		end,
	})

	input:mount()

	input:map("n", "<Esc>", function()
		layout:unmount()
		input:unmount()
	end, { noremap = true })
	input:map("n", "q", function()
		layout:unmount()
		input:unmount()
	end, { noremap = true })
end

function ShowDeleteConnectionPopup(layout)
	layout:hide()
	local popup_options = {
		relative = "editor",
		position = "50%",
		size = {
			width = "50%",
			height = 1,
		},
		border = {
			style = "rounded",
			text = {
				top = "[ Delete Connection ]",
				top_align = "left",
			},
		},
	}

	local input = Input(popup_options, {
		on_submit = function(value)
			if #value > 0 then
				local _, err = vim.fn["SimpananDeleteConnection"](value)
				if err ~= nil then
					util.print_red(err)
					return
				else
					get_connections()

					local popup = newConnectionPopup()
					AddPopupHooks(popup, layout)

					layout:update(Layout.Box({
						Layout.Box(popup, { size = "100%" }),
					}))
				end
			end

			layout:show()
		end,
	})

	input:mount()

	input:map("n", "<Esc>", function()
		layout:unmount()
		input:unmount()
	end, { noremap = true })
	input:map("n", "q", function()
		layout:unmount()
		input:unmount()
	end, { noremap = true })
end

function M.list_connections()
	if M.conn_count == 0 then
		get_connections()
	end

	local popup = newConnectionPopup()
	local layout = Layout(
		{
			relative = "editor",
			position = "50%",
			size = {
				width = "50%",
				height = "50%",
			},
		},
		Layout.Box({
			Layout.Box(popup, { size = "100%" }),
		})
	)

	layout:mount()
	AddPopupHooks(popup, layout)
end

function M.execute()
	local lines = require("simpanan.util").get_visual_selection_text()
	local req = ""
	if lines ~= nil then
		for _, l in ipairs(lines) do
			req = req .. "::" .. l
		end
	end

	local opts = ""
	for k, v in pairs(M.opts) do
		opts = opts .. "::" .. k .. "=" .. tostring(v)
	end
	local res, err = vim.fn["SimpananRunQuery"](req, opts)
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

		-- vim.api.nvim_buf_set_option(split.bufnr, "filetype", "json")
		vim.bo[split.bufnr].filetype = "json"

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

function M.setup(opts)
	M.opts = vim.tbl_deep_extend("force", default_opts, (opts or {}))
end

return M
