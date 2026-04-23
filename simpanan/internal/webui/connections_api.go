package webui

import (
	"encoding/json"
	"net/http"
	"strings"

	"simpanan/internal"
	"simpanan/internal/common"
)

type connectionDTO struct {
	Label string `json:"label"`
	URI   string `json:"uri"`
}

type connectionListResponse struct {
	Connections []connectionDTO `json:"connections"`
}

type addConnectionRequest struct {
	Label string `json:"label"`
	URI   string `json:"uri"`
}

type deleteConnectionRequest struct {
	Label string `json:"label"`
}

// registerConnectionRoutes mounts the connection-management surface.
//   GET    /api/connections        → list
//   POST   /api/connections        → add (body: {label, uri})
//   DELETE /api/connections        → delete (body: {label})
//
// Maps to the spec's existing AddConnection / DeleteConnection rules
// in simpanan.allium plus the webui's OpenConnectionsPopup flow.
// Edit is deliberately not implemented (per scoping decision).
func (s *Server) registerConnectionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/connections", s.handleConnections)
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListConnections(w, r)
	case http.MethodPost:
		s.handleAddConnection(w, r)
	case http.MethodDelete:
		s.handleDeleteConnection(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleListConnections(w http.ResponseWriter, _ *http.Request) {
	conns, err := internal.GetConnectionList()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	dtos := make([]connectionDTO, 0, len(conns))
	for _, c := range conns {
		dtos = append(dtos, connectionDTO{Label: c.Key, URI: string(c.URI)})
	}
	writeJSON(w, http.StatusOK, connectionListResponse{Connections: dtos})
}

func (s *Server) handleAddConnection(w http.ResponseWriter, r *http.Request) {
	var req addConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	req.URI = strings.TrimSpace(req.URI)
	if err := internal.AddConnection(req.Label, req.URI); err != nil {
		// User-input validation errors come back as plain errors;
		// return them as 400 so the UI can show the message inline.
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	uri := string(common.URI(req.URI))
	writeJSON(w, http.StatusOK, connectionDTO{Label: req.Label, URI: uri})
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	var req deleteConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if err := internal.DeleteConnection(strings.TrimSpace(req.Label)); err != nil {
		// 404 if the label is unknown, 400 otherwise. Cheap heuristic
		// on the message — the underlying Go errors are bare strings.
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "does not exist") {
			status = http.StatusNotFound
		}
		writeJSON(w, status, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "label": req.Label})
}
