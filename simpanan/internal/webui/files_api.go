package webui

import (
	"encoding/json"
	"errors"
	"net/http"
)

// writeJSON writes v as a JSON body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNotSimpFile), errors.Is(err, ErrEmptyPath):
		status = http.StatusBadRequest
	case errors.Is(err, ErrPathNotFound), errors.Is(err, ErrFileNotOpen):
		status = http.StatusNotFound
	case errors.Is(err, ErrAlreadyOpen):
		status = http.StatusConflict
	}
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

type filesListResponse struct {
	Files  []OpenFile `json:"files"`
	Active string     `json:"active"`
}

type openRequest struct {
	Path string `json:"path"`
}

type closeRequest struct {
	Path string `json:"path"`
}

type saveRequest struct {
	Path string `json:"path"`
}

type editRequest struct {
	Path             string `json:"path"`
	BufferContents   string `json:"buffer_contents"`
	CursorByteOffset int    `json:"cursor_byte_offset"`
}

type switchActiveRequest struct {
	Path string `json:"path"`
}

// registerFileRoutes wires the file-operations HTTP surface onto mux.
// Each handler maps to a webui.allium rule:
//   GET    /api/files                → list open_files + active
//   GET    /api/files/get?path=…     → one OpenFile's full state
//   POST   /api/files/open           → OpenFileInWebui
//   POST   /api/files/close          → CloseFile
//   POST   /api/files/save           → SaveFile
//   POST   /api/files/edit           → EditBuffer
//   POST   /api/files/switch-active  → SwitchActiveFile
func (s *Server) registerFileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/files", s.handleListFiles)
	mux.HandleFunc("/api/files/get", s.handleGetFile)
	mux.HandleFunc("/api/files/open", s.handleOpen)
	mux.HandleFunc("/api/files/close", s.handleClose)
	mux.HandleFunc("/api/files/save", s.handleSave)
	mux.HandleFunc("/api/files/edit", s.handleEdit)
	mux.HandleFunc("/api/files/switch-active", s.handleSwitchActive)
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, filesListResponse{
		Files:  s.buffers.List(),
		Active: s.buffers.Active(),
	})
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	f, ok := s.buffers.Get(path)
	if !ok {
		writeError(w, ErrFileNotOpen)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req openRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	f, err := s.buffers.Open(req.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req closeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if err := s.buffers.Close(req.Path); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "closed"})
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if err := s.buffers.Save(req.Path); err != nil {
		writeError(w, err)
		return
	}
	f, _ := s.buffers.Get(req.Path)
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req editRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	f, err := s.buffers.Edit(req.Path, req.BufferContents, req.CursorByteOffset)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleSwitchActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req switchActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if err := s.buffers.SwitchActive(req.Path); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"active": req.Path})
}
