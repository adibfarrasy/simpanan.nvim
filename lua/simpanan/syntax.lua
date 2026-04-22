-- Simpanan filetype syntax wiring.
--
-- For each registered connection, inject Neovim's bundled syntax for the
-- matching dialect into stages whose label resolves to that connection:
--
--   * Postgres / MySQL  -> syntax/sql.vim  (oracle flavour + supplement)
--   * MongoDB           -> syntax/javascript.vim (mongo shell is JS-based)
--
-- Redis and jq stages have no bundled syntax and keep the generic
-- highlighting from syntax/simpanan.vim.
--
-- Called from:
--   * ftplugin/simpanan.lua on BufRead/BufNewFile and BufEnter
--   * lua/simpanan/init.lua after add/delete, so already-open buffers
--     refresh without requiring :edit.

local M = {}

local connections_path = vim.fn.expand("~/.local/share/nvim/simpanan_connections.json")

-- Per-buffer list of region group names we've created, so we can clear
-- them before rebuilding on refresh (otherwise regions accumulate and
-- stale labels keep highlighting after the user deletes them).
local created_groups_by_buf = {}

-- Module-level flag: the SQL and JS syntax files only need including
-- once per buffer. Subsequent refreshes just add/remove regions.
local includes_installed_by_buf = {}

-- uri_kind returns "sql", "js", or nil based on the URI scheme. Labels
-- whose URI we don't understand get nil and fall back to generic
-- simpanan highlighting.
local function uri_kind(uri)
	if uri:match("^postgres://")
		or uri:match("^postgresql://")
		or uri:match("^mysql://") then
		return "sql"
	end
	if uri:match("^mongodb://")
		or uri:match("^mongodb%+srv://") then
		return "js"
	end
	return nil
end

-- Read the connections file and group label names by the syntax we want
-- to inject. Missing / malformed file => empty groups.
local function read_labels_by_kind()
	local groups = { sql = {}, js = {} }

	local f = io.open(connections_path, "r")
	if not f then
		return groups
	end
	local body = f:read("*a")
	f:close()
	if not body or body == "" then
		return groups
	end

	local ok, parsed = pcall(vim.json.decode, body)
	if not ok or type(parsed) ~= "table" then
		return groups
	end

	for _, entry in ipairs(parsed) do
		local key = entry.key_name
		local uri = entry.uri
		if type(key) == "string" and type(uri) == "string" then
			local kind = uri_kind(uri)
			if kind then
				table.insert(groups[kind], key)
			end
		end
	end
	return groups
end

-- Escape a connection label for use inside a Vim regex. Labels are
-- constrained by the registry (no '>', no whitespace around them) but
-- could still contain regex metacharacters like '.', '-', '+'.
local function vim_regex_escape(s)
	return (s:gsub("([%.%-%+%*%?%[%]%(%)%^%$%|%\\])", "\\%1"))
end

-- Wraps `syntax include` so the sourced file's
-- `if exists("b:current_syntax") | finish` guard does not short-circuit
-- it. Unlet + include + restore must be issued as a single Ex-command
-- block; separate `vim.cmd` calls don't execute with the timing
-- `:syntax include` expects under lazy-loaded runtimes, and the
-- include ends up as a no-op.
local function include_syntax(cluster, file)
	vim.api.nvim_exec2(string.format([[
		let g:_simpanan_prev_syn = get(b:, 'current_syntax', '')
		unlet! b:current_syntax
		syntax include @%s %s
		if g:_simpanan_prev_syn != ''
			let b:current_syntax = g:_simpanan_prev_syn
		endif
		unlet g:_simpanan_prev_syn
	]], cluster, file), {})
end

-- Install the @simpananSQL cluster and supplement sqloracle with the
-- modern SQL keywords it misses. Idempotent per buffer.
local function install_sql_syntax()
	include_syntax("simpananSQL", "syntax/sql.vim")

	-- Supplement sqloracle: it lacks LIMIT / OFFSET / RETURNING / ILIKE
	-- and has no rule that matches numbers.
	vim.cmd([[
		syntax case ignore
		" PG/MySQL keywords sqloracle omits.
		syntax keyword simpSqlKeyword contained limit offset returning ilike
		syntax keyword simpSqlKeyword contained window partition over
		syntax keyword simpSqlKeyword contained array unnest coalesce nullif greatest least
		" Metadata / DDL-adjacent keywords (MySQL-style SHOW / DESCRIBE,
		" plus common EXPLAIN variants).
		syntax keyword simpSqlKeyword contained show tables databases columns indexes
		syntax keyword simpSqlKeyword contained describe desc explain use schemas
		syntax match   simpSqlNumber  "\<\d\+\(\.\d\+\)\?\>" contained
		syntax cluster simpananSQL add=simpSqlKeyword,simpSqlNumber
		syntax case match
	]])

	-- sqloracle links sqlKeyword -> sqlSpecial -> Special, which most
	-- colourschemes render pale. Re-link to canonical groups so SQL
	-- keywords get normal-keyword treatment. `highlight!` overrides any
	-- prior link.
	vim.cmd([[
		highlight! link sqlKeyword     Keyword
		highlight! link sqlStatement   Statement
		highlight! link sqlOperator    Operator
		highlight! link sqlType        Type
		highlight! link sqlFunction    Function
		highlight! link sqlSpecial     Constant
		highlight! link sqlNumber      Number
		highlight! link sqlString      String
		highlight! link sqlComment     Comment
		highlight! link simpSqlKeyword Keyword
		highlight! link simpSqlNumber  Number
	]])
end

-- Install the @simpananJS cluster for Mongo shell stages. Idempotent.
local function install_js_syntax()
	include_syntax("simpananJS", "syntax/javascript.vim")

	-- The bundled javascript.vim does not have a rule that tags
	-- unquoted object keys like `status:` or `_id:`. Add a contained
	-- match so `key:` inside a Mongo stage renders as a property.
	vim.cmd([[
		syntax match   simpJsObjectKey "\<[A-Za-z_$][A-Za-z0-9_$]*\ze\s*:" contained
		syntax cluster simpananJS add=simpJsObjectKey
	]])

	vim.cmd([[
		highlight! link simpJsObjectKey Identifier
	]])
end

-- Core entry point: wipe prior regions for `bufnr`, rebuild from the
-- current registry state.
function M.setup_sql_regions(bufnr)
	bufnr = bufnr or vim.api.nvim_get_current_buf()
	if not vim.api.nvim_buf_is_valid(bufnr) then
		return
	end

	vim.api.nvim_buf_call(bufnr, function()
		-- Clear regions installed by a previous call for this buffer.
		local prior = created_groups_by_buf[bufnr] or {}
		for _, g in ipairs(prior) do
			pcall(vim.cmd, "syntax clear " .. g)
		end
		created_groups_by_buf[bufnr] = {}

		local labels = read_labels_by_kind()
		if #labels.sql == 0 and #labels.js == 0 then
			return
		end

		-- Stage regions span multiple lines; vim's default syntax sync
		-- does not reliably back up far enough to pick up the start of
		-- a region when the cursor lands deep in it. Full re-sync from
		-- the top of the buffer is cheap at the sizes this plugin sees.
		vim.cmd([[syntax sync fromstart]])

		-- Include SQL / JS syntax once per buffer.
		local installed = includes_installed_by_buf[bufnr] or {}
		if not installed.sql and #labels.sql > 0 then
			install_sql_syntax()
			installed.sql = true
		end
		if not installed.js and #labels.js > 0 then
			install_js_syntax()
			installed.js = true
		end
		includes_installed_by_buf[bufnr] = installed

		-- Create a region per label. Stages span from a `label>` line
		-- to (but not including) the next stage's `label>` line, or to
		-- end-of-file.
		local function make_region(label, cluster, group_prefix)
			local esc = vim_regex_escape(label)
			local group = group_prefix .. label:gsub("[^%w]", "_")
			vim.cmd(string.format(
				[[syntax region %s start="^\s*%s>" end="^\s*\S\+>"me=s-1 end="\%%$" keepend contains=@%s,simpananConnLabel,simpananComment,simpananPlaceholder]],
				group, esc, cluster
			))
			table.insert(created_groups_by_buf[bufnr], group)
		end

		for _, label in ipairs(labels.sql) do
			make_region(label, "simpananSQL", "simpananSQLStage_")
		end
		for _, label in ipairs(labels.js) do
			make_region(label, "simpananJS", "simpananJSStage_")
		end
	end)
end

-- Refresh every loaded *.simp buffer. Called by init.lua after a
-- connection is added or deleted so already-open buffers reflect the
-- new registry state immediately.
function M.refresh_all()
	for _, bufnr in ipairs(vim.api.nvim_list_bufs()) do
		if vim.api.nvim_buf_is_loaded(bufnr)
			and vim.bo[bufnr].filetype == "simpanan" then
			M.setup_sql_regions(bufnr)
		end
	end
end

return M
