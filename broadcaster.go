// -*- tab-width:2 -*-

package sim

import (
	"sync"

	count "github.com/jayalane/go-counter"
)

// This file is a one to all broadcaster that is selectable.
// by GPT4.

// Broadcaster allows broadcasting to multiple goroutines.
type Broadcaster struct {
	subscribers []chan *sync.WaitGroup
	mu          sync.Mutex
}

// NewBroadcaster creates a new Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{}
}

// Subscribe adds a new subscriber.
func (b *Broadcaster) Subscribe() chan *sync.WaitGroup {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *sync.WaitGroup, smallChannelSize)
	b.subscribers = append(b.subscribers, ch)

	return ch
}

// Broadcast sends the signal to all subscribers.
func (b *Broadcaster) Broadcast(wg *sync.WaitGroup) {
	b.mu.Lock()
	defer b.mu.Unlock()
	count.IncrSync("broadcaster_broadcast_input")

	for _, ch := range b.subscribers {
		wg.Add(1)
		ml.La("Adding one to Waitgroup")
		select {
		case ch <- wg:
			count.IncrSync("broadcaster_broadcast_output")
		default:
			wg.Done()
			ml.La("Removing one to Waitgroup")
			ml.La("Dropped subscriber select")
			count.IncrSync("broadcaster_drop_output")
		}
	}
}
