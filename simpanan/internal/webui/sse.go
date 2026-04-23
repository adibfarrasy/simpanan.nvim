package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// handleEvents serves a long-lived Server-Sent Events stream.
// The browser opens this once per tab; the server pushes every
// EventBus event down the connection so the tab stays in sync
// with the canonical server state.
//
// SSE was chosen over WebSocket because the wire is one-way (server
// → browser); browser-to-server traffic uses the existing /api/files
// POST endpoints. SSE has no extra dependency, automatic reconnection
// in browsers, and is trivial to test with a regular HTTP client.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Tell the browser the stream is intentionally long-lived.
	w.WriteHeader(http.StatusOK)

	ch, unsubscribe := s.events.Subscribe()
	defer unsubscribe()

	// Initial comment so the browser sees an open connection.
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(e)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
