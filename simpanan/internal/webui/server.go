// Package webui implements the long-lived browser-based client for
// simpanan, specified in specs/webui.allium. It is invoked through
// cmd/simpanan-webui and shares the simpanan internal package with the
// existing Neovim rplugin entry point so all backend logic
// (connections, queries, autocomplete) is reused.
package webui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// DefaultPort is the SIMP phone-keypad mapping (S=7 I=4 M=6 P=7).
// Pinned by the webui spec's webui_port config and not currently
// overridable — when an override is wanted, surface it through a CLI
// flag at the cmd/ layer.
const DefaultPort = 7467

// ServerStatus mirrors the WebuiServerStatus enum in webui.allium.
type ServerStatus string

const (
	StatusStarting     ServerStatus = "starting"
	StatusRunning      ServerStatus = "running"
	StatusShuttingDown ServerStatus = "shutting_down"
)

// Server is the long-lived webui process. At most one is constructed
// per simpanan-webui invocation (the SingleWebuiInstance invariant
// from the spec is enforced by the OS port-bind: a second launch on
// the same port aborts).
type Server struct {
	port   int
	mu     sync.RWMutex
	status ServerStatus

	httpServer *http.Server
	listener   net.Listener

	buffers *BufferStore
}

// NewServer constructs a Server in StatusStarting. It does not bind
// the port; call Start to do that.
func NewServer(port int) *Server {
	return &Server{
		port:    port,
		status:  StatusStarting,
		buffers: NewBufferStore(),
	}
}

// Status reports the current ServerStatus.
func (s *Server) Status() ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// setStatus transitions through the spec's status graph.
func (s *Server) setStatus(next ServerStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = next
}

// Start binds the configured port and serves until shutdown is
// requested via Ctrl-C (SIGINT) or SIGTERM. Spec rule LaunchWebui:
// hostile-fail when the port is already in use; matching spec rule
// LaunchAborted: emit a one-line abort reason to stderr and exit
// nonzero. The terminal-print of the live URL is the
// BecomeRunning rule's domain obligation.
func (s *Server) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("LaunchAborted: cannot bind %s: %w", addr, err)
	}
	s.listener = ln

	mux := http.NewServeMux()
	s.registerRoutes(mux)
	s.registerFileRoutes(mux)

	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.setStatus(StatusRunning)
	fmt.Fprintf(os.Stderr, "simpanan webui running at http://localhost:%d\n", s.port)
	fmt.Fprintln(os.Stderr, "press Ctrl-C to stop.")

	serveErr := make(chan error, 1)
	go func() {
		err := s.httpServer.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sig:
		// User-initiated shutdown — graceful. Spec rule ShutdownWebui.
		return s.shutdown()
	case err := <-serveErr:
		// Unexpected server failure. Treat as fatal.
		return err
	}
}

// shutdown transitions to StatusShuttingDown, gives in-flight HTTP
// requests up to 5s to finish, and returns. The recovery-file flush
// promised by ShutdownWebui will be added in a later milestone.
func (s *Server) shutdown() error {
	s.setStatus(StatusShuttingDown)
	fmt.Fprintln(os.Stderr, "\nshutting down…")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// registerRoutes wires the HTTP handlers. Real handlers arrive in
// later milestones; M1 ships only the placeholder root and a health
// endpoint useful for debugging "is something on this port already
// my webui?".
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, indexPlaceholder)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"server":"simpanan-webui","status":"`+string(s.Status())+`","port":`+strconv.Itoa(s.port)+`}`)
	})
}

const indexPlaceholder = `<!doctype html>
<html>
  <head><meta charset="utf-8"><title>simpanan webui</title></head>
  <body>
    <h1>simpanan webui</h1>
    <p>The webui is running. Editor UI lands in a later milestone.</p>
  </body>
</html>
`
