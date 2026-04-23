package webui

import (
	"encoding/json"
	"net/http"
	"strings"

	"simpanan/internal"
)

type executeRequest struct {
	// Selection is the raw selected text. The handler splits it on
	// newlines and feeds it to RunPipeline.
	Selection string `json:"selection"`
	// Opts are key=value strings the same shape as the rplugin's
	// args[1] payload. Optional; may be nil.
	Opts []string `json:"opts,omitempty"`
}

type executeResponse struct {
	// Result is the rendered output: pretty-printed JSON for read
	// queries, the adapter-specific status message for writes, or an
	// "Error: …" string when a stage fails (the existing semantics
	// of HandleRunQuery / RunPipeline are preserved verbatim so both
	// clients see identical output).
	Result string `json:"result"`
}

// registerExecuteRoutes mounts /api/execute. Spec rule
// ExecuteSelection: requires selected_text.length > 0 (no auto-detect),
// forwards to the existing pipeline path.
func (s *Server) registerExecuteRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/execute", s.handleExecute)
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req executeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	selection := strings.TrimSpace(req.Selection)
	if selection == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "selection is empty; explicit selection is required"})
		return
	}

	stageLines := strings.Split(req.Selection, "\n")
	result, err := internal.RunPipeline(stageLines, req.Opts)
	if err != nil {
		// RunPipeline folds adapter and validation errors into the
		// returned string with an "Error: " prefix and no Go-side
		// error. A non-nil err here means the call itself failed
		// catastrophically (e.g. config parse) — surface it as 500.
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, executeResponse{Result: result})
}
