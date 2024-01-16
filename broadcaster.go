// -*- tab-width:2 -*-

package sim

import (
	"sync"
)

// This file is a one to all broadcaster that is selectable.
// by GPT4.

// Broadcaster allows broadcasting to multiple goroutines.
type Broadcaster struct {
	subscribers []chan bool
	mu          sync.Mutex
}

// NewBroadcaster creates a new Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{}
}

// Subscribe adds a new subscriber.
func (b *Broadcaster) Subscribe() chan bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan bool, 1) // Buffered channel
	b.subscribers = append(b.subscribers, ch)
	return ch
}

// Broadcast sends the signal to all subscribers.
func (b *Broadcaster) Broadcast() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- true:
		default:
		}
	}
}
