// Package realtime implements a small in-memory pub/sub hub used to push
// task events to connected SSE clients.
package realtime

import "sync"

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type Subscriber struct {
	UserID string
	Admin  bool
	Ch     chan Event
}

type Hub struct {
	mu   sync.RWMutex
	subs map[*Subscriber]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: map[*Subscriber]struct{}{}}
}

func (h *Hub) Subscribe(userID string, admin bool) *Subscriber {
	s := &Subscriber{UserID: userID, Admin: admin, Ch: make(chan Event, 16)}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

func (h *Hub) Unsubscribe(s *Subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
}

// Publish delivers the event to every connection belonging to the task owner
// and to all connected admins. Subscribers with a full buffer are skipped
// rather than blocked on.
func (h *Hub) Publish(ownerID string, ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		if s.UserID == ownerID || s.Admin {
			select {
			case s.Ch <- ev:
			default:
			}
		}
	}
}
