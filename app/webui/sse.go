package webui

import (
	"sync"
)

// Event is a server-sent event payload.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// broker fans out events to all subscribers.
type broker struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
}

func newBroker() *broker {
	return &broker{subscribers: map[chan Event]struct{}{}}
}

func (b *broker) subscribe() chan Event {
	ch := make(chan Event, 256)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *broker) publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- e:
		default: // drop for slow subscribers
		}
	}
}
