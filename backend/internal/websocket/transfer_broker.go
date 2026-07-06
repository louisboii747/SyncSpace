package websocket

import (
	"sync"
	"sync/atomic"

	"github.com/louisboii747/syncspace/backend/internal/transfer"
)

type TransferBroker struct {
	mu          sync.Mutex
	subscribers map[uint64]chan transfer.Event
	nextID      atomic.Uint64
}

func NewTransferBroker() *TransferBroker {
	return &TransferBroker{subscribers: make(map[uint64]chan transfer.Event)}
}
func (b *TransferBroker) Publish(event transfer.Event) {
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
func (b *TransferBroker) Subscribe() (<-chan transfer.Event, func()) {
	id := b.nextID.Add(1)
	queue := make(chan transfer.Event, subscriberBuffer)
	b.mu.Lock()
	b.subscribers[id] = queue
	b.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			if subscriber, ok := b.subscribers[id]; ok {
				delete(b.subscribers, id)
				close(subscriber)
			}
			b.mu.Unlock()
		})
	}
	return queue, cancel
}
