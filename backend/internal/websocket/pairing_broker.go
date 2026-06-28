package websocket

import (
	"sync"
	"sync/atomic"

	"github.com/louisboii747/syncspace/backend/internal/pairing"
)

// PairingBroker fans pairing events out to connected clients.
type PairingBroker struct {
	mu          sync.Mutex
	subscribers map[uint64]chan pairing.Event
	nextID      atomic.Uint64
}

// NewPairingBroker constructs an empty pairing event broker.
func NewPairingBroker() *PairingBroker {
	return &PairingBroker{subscribers: make(map[uint64]chan pairing.Event)}
}

// Publish implements pairing.EventPublisher. Slow clients are disconnected so
// they can reconnect and replay the current trusted-device snapshot.
func (b *PairingBroker) Publish(event pairing.Event) {
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

// Subscribe registers a pairing event queue and an idempotent cancellation
// function.
func (b *PairingBroker) Subscribe() (<-chan pairing.Event, func()) {
	id := b.nextID.Add(1)
	queue := make(chan pairing.Event, subscriberBuffer)
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
