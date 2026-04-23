package internal

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// HandleSuggest is the rplugin entry point for autocomplete. Callers
// pass the full buffer text as args[0] and the cursor byte offset as
// args[1]. Returns a JSON-encoded array of Suggestions.
//
// Signature chosen to match the existing rplugin handler convention
// (args []string → (string, error)). A Lua caller consumes this via
// vim.json.decode on the returned string.
func HandleSuggest(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("SimpananSuggest: expected buffer_text and cursor_pos args")
	}
	bufferText := args[0]
	cursorPos, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("SimpananSuggest: cursor_pos must be an integer, got %q", args[1])
	}
	suggestions := SuggestForBuffer(bufferText, cursorPos)
	data, err := json.Marshal(suggestions)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
