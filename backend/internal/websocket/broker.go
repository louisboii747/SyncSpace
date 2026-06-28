// Package websocket delivers discovery registry events to connected clients.
package websocket

import (
	"sync"
	"sync/atomic"

	"github.com/louisboii747/syncspace/backend/internal/models"
)

const subscriberBuffer = 64

// Broker fans discovery events out to independent client queues.
type Broker struct {
	mu          sync.Mutex
	subscribers map[uint64]chan models.DiscoveryEvent
	nextID      atomic.Uint64
}

// NewBroker constructs an empty event broker.
func NewBroker() *Broker {
	return &Broker{subscribers: make(map[uint64]chan models.DiscoveryEvent)}
}

// Publish sends an event to all current subscribers. A client that cannot keep
// up with a full queue is disconnected so it can reconnect and obtain a fresh
// registry snapshot rather than silently miss state transitions.
func (b *Broker) Publish(event models.DiscoveryEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, subscriber := range b.subscribers {
		select {
		case subscriber <- event:
		default:
			close(subscriber)
			delete(b.subscribers, id)
		}
	}
}

// Subscribe registers an event queue and returns an idempotent cancellation
// function.
func (b *Broker) Subscribe() (<-chan models.DiscoveryEvent, func()) {
	id := b.nextID.Add(1)
	queue := make(chan models.DiscoveryEvent, subscriberBuffer)
	b.mu.Lock()
	b.subscribers[id] = queue
	b.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			if subscriber, found := b.subscribers[id]; found {
				delete(b.subscribers, id)
				close(subscriber)
			}
			b.mu.Unlock()
		})
	}
	return queue, cancel
}
