package webui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventBus_PublishReachesAllSubscribers(t *testing.T) {
	bus := NewEventBus()
	a, unsubA := bus.Subscribe()
	defer unsubA()
	b, unsubB := bus.Subscribe()
	defer unsubB()

	bus.Publish(Event{Type: EventBufferUpdated, Payload: "hello"})

	for _, ch := range []<-chan Event{a, b} {
		select {
		case got := <-ch:
			assert.Equal(t, EventBufferUpdated, got.Type)
		case <-time.After(time.Second):
			t.Fatal("subscriber did not receive event")
		}
	}
}

func TestEventBus_UnsubscribeRemovesAndCloses(t *testing.T) {
	bus := NewEventBus()
	ch, unsub := bus.Subscribe()
	assert.Equal(t, 1, bus.SubscriberCount())
	unsub()
	assert.Equal(t, 0, bus.SubscriberCount())

	// Channel is closed.
	_, ok := <-ch
	assert.False(t, ok)

	// Double unsubscribe is a no-op (no panic).
	unsub()
}

func TestEventBus_DropsOnSlowSubscriber(t *testing.T) {
	bus := NewEventBus()
	_, unsub := bus.Subscribe()
	defer unsub()

	// Fill the buffer (16) and overrun.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			bus.Publish(Event{Type: "x", Payload: i})
		}
	}()
	wg.Wait()
	// If publish blocked we would never reach here within the timeout.
}

// readSSEEvent pulls a single SSE event payload from the response body.
// Returns the JSON portion of the first `data:` line; returns "" on
// timeout.
func readSSEEvent(t *testing.T, scanner *bufio.Scanner, deadline time.Duration) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				done <- strings.TrimPrefix(line, "data: ")
				return
			}
		}
		done <- ""
	}()
	select {
	case s := <-done:
		return s
	case <-time.After(deadline):
		return ""
	}
}

func TestSSE_BroadcastsBufferUpdateToConnectedTab(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	// Create a real .simp file so the open call succeeds.
	simpPath := filepath.Join(t.TempDir(), "a.simp")
	assert.NoError(t, os.WriteFile(simpPath, []byte("|pg> SELECT 1"), 0644))

	// Subscribe (long-lived GET) in a goroutine.
	resp, err := http.Get(base + "/api/events")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	scanner := bufio.NewScanner(resp.Body)

	// Trigger an open event and then an edit event.
	_, _ = postJSON(t, base, "/api/files/open", openRequest{Path: simpPath})
	_, _ = postJSON(t, base, "/api/files/edit", editRequest{
		Path: simpPath, BufferContents: "|pg> SELECT 99", CursorByteOffset: 13,
	})

	// First we should see file_opened, then buffer_updated.
	first := readSSEEvent(t, scanner, 2*time.Second)
	assert.NotEqual(t, "", first, "expected first SSE event")
	var ev Event
	assert.NoError(t, json.Unmarshal([]byte(first), &ev))
	assert.Equal(t, EventFileOpened, ev.Type)

	second := readSSEEvent(t, scanner, 2*time.Second)
	assert.NotEqual(t, "", second, "expected second SSE event")
	assert.NoError(t, json.Unmarshal([]byte(second), &ev))
	assert.Equal(t, EventBufferUpdated, ev.Type)
}

func TestSSE_DisconnectUnsubscribes(t *testing.T) {
	base, srv, stop := startTestServer(t)
	defer stop()

	resp, err := http.Get(base + "/api/events")
	assert.NoError(t, err)

	// Wait for the subscription to register.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && srv.events.SubscriberCount() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, 1, srv.events.SubscriberCount())

	resp.Body.Close()

	// Wait for the handler to detect the disconnect and unsubscribe.
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && srv.events.SubscriberCount() != 0 {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, 0, srv.events.SubscriberCount(), "tab disconnect must release the subscription")
}

func TestSSE_RejectsNonGet(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	resp, err := http.Post(base+"/api/events", "application/json", strings.NewReader(""))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

// silence unused fmt import in some build configs
var _ = fmt.Sprint
