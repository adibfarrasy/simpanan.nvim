package webui

import (
	"sync"
)

// EventBus is a fan-out broker for buffer-broadcast events. The HTTP
// SSE handler subscribes a channel; mutating handlers (Edit, Save,
// Open, Close, SwitchActive) publish through the bus so every
// connected browser tab observes the same state.
//
// Spec rule BroadcastBufferToTabs: every accepted edit produces
// exactly one notification to every other tab. The bus delivers to
// ALL subscribers including the originator — tabs are expected to
// reconcile based on their own latest local state and silently
// ignore self-echoes if needed (kept on the wire for simplicity).
//
// Slow consumers are dropped (non-blocking send): a tab that cannot
// keep up will miss events rather than stall the whole server. The
// HTTP handler can detect this via channel close on context done.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

// Event is a single broadcast payload. Type discriminates the body
// so the same channel can carry future event kinds (file_opened,
// file_closed, active_switched, …) without breaking subscribers.
type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Event type discriminators. Hand-edit this list as new events are
// added. The browser side switches on Type.
const (
	EventBufferUpdated  = "buffer_updated"
	EventFileOpened     = "file_opened"
	EventFileClosed     = "file_closed"
	EventActiveSwitched = "active_switched"
	EventFileSaved      = "file_saved"
)

func NewEventBus() *EventBus {
	return &EventBus{subscribers: map[chan Event]struct{}{}}
}

// Subscribe returns a buffered channel that receives every published
// event. The returned unsubscribe function MUST be called by the
// caller (typically deferred) so the bus does not leak goroutines.
func (b *EventBus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	unsubscribe := func() {
		b.mu.Lock()
		if _, ok := b.subscribers[ch]; ok {
			delete(b.subscribers, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, unsubscribe
}

// Publish fans an event out to every subscriber. Non-blocking: if a
// subscriber's buffer is full, the event is dropped for that
// subscriber rather than blocking publish.
func (b *EventBus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- e:
		default:
			// drop for this slow subscriber
		}
	}
}

// SubscriberCount is exposed for tests.
func (b *EventBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
