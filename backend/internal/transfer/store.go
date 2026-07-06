package transfer

import "context"

// Store is the durable boundary used by the transfer state machine. All
// implementations must make SaveTransfer atomic with its queue/history mirrors.
type Store interface {
	SaveTransfer(context.Context, Transfer) error
	GetTransfer(context.Context, string) (Transfer, error)
	ListTransfers(context.Context) ([]Transfer, error)
	DeleteHistory(context.Context) error
	SaveChunk(context.Context, Chunk) error
	ListChunks(context.Context, string) ([]Chunk, error)
}
