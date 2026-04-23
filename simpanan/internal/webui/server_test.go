package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// freePort grabs an OS-assigned high port so tests do not collide
// with anything else and do not clash with the real DefaultPort.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// startTestServer spins up a Server on a free port in a goroutine,
// waits for it to be reachable, and returns its base URL plus a
// cleanup function that stops the server.
func startTestServer(t *testing.T) (string, *Server, func()) {
	t.Helper()
	port := freePort(t)
	srv := NewServer(port)

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start() }()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if srv.Status() != StatusRunning {
		t.Fatalf("server failed to reach running status; got %s", srv.Status())
	}

	cleanup := func() {
		_ = srv.shutdown()
		select {
		case <-startErr:
		case <-time.After(2 * time.Second):
			t.Fatalf("server did not stop within 2s")
		}
	}
	return base, srv, cleanup
}

func TestServer_StartStopHealthIsReachable(t *testing.T) {
	base, srv, stop := startTestServer(t)
	defer stop()

	resp, err := http.Get(base + "/health")
	assert.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var payload map[string]interface{}
	assert.NoError(t, json.Unmarshal(body, &payload))
	assert.Equal(t, "simpanan-webui", payload["server"])
	assert.Equal(t, string(StatusRunning), payload["status"])

	// Status struct also reflects running.
	assert.Equal(t, StatusRunning, srv.Status())
}

func TestServer_RootServesPlaceholderHtml(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	resp, err := http.Get(base + "/")
	assert.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "simpanan webui")
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
}

func TestServer_UnknownPathIs404(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	resp, err := http.Get(base + "/nonexistent")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_HostileFailWhenPortInUse(t *testing.T) {
	// Hold a port to simulate "another instance is here".
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	srv := NewServer(port)
	err = srv.Start()
	assert.Error(t, err, "Start must fail when port is bound")
	assert.True(t, strings.HasPrefix(err.Error(), "LaunchAborted:"),
		"error must reference the LaunchAborted spec rule; got %q", err.Error())
	assert.Equal(t, StatusStarting, srv.Status(),
		"server must remain in starting status when bind fails")
}

func TestServer_ShutdownTransitionsStatus(t *testing.T) {
	_, srv, stop := startTestServer(t)
	stop()
	assert.Equal(t, StatusShuttingDown, srv.Status())
}
