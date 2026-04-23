package webui

import (
	"encoding/json"
	"net/http"

	"simpanan/internal"
)

type suggestRequest struct {
	BufferText       string `json:"buffer_text"`
	CursorByteOffset int    `json:"cursor_byte_offset"`
}

type suggestResponse struct {
	Suggestions []internal.Suggestion `json:"suggestions"`
}

// registerSuggestRoutes mounts /api/suggest. Wraps the existing
// SuggestForBuffer entry point in a webui-shaped JSON contract so the
// browser does not have to deal with the rplugin's []string interface.
func (s *Server) registerSuggestRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/suggest", s.handleSuggest)
}

func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req suggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	suggestions := internal.SuggestForBuffer(req.BufferText, req.CursorByteOffset)
	writeJSON(w, http.StatusOK, suggestResponse{Suggestions: suggestions})
}
