local m = require("simpanan.cmp")
local orig = m.complete
m.complete = function(self, ...)
	vim.notify("simpanan complete fired")
	return orig(self, ...)
end
