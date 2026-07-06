package websocket

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	"github.com/louisboii747/syncspace/backend/internal/transfer"
)

type TransferLister interface {
	List(context.Context) ([]transfer.Transfer, error)
}
type TransferHandler struct {
	broker    *TransferBroker
	transfers TransferLister
	logger    *slog.Logger
	upgrader  gorilla.Upgrader
}

func NewTransferHandler(broker *TransferBroker, transfers TransferLister, logger *slog.Logger) *TransferHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &TransferHandler{broker: broker, transfers: transfers, logger: logger, upgrader: gorilla.Upgrader{HandshakeTimeout: 10 * time.Second, ReadBufferSize: 1024, WriteBufferSize: 8192, CheckOrigin: sameHostOrigin}}
}
func (h *TransferHandler) Serve(c *gin.Context) {
	connection, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Warn("Transfer WebSocket upgrade failed", "error", err)
		return
	}
	defer connection.Close()
	events, unsubscribe := h.broker.Subscribe()
	defer unsubscribe()
	items, err := h.transfers.List(c.Request.Context())
	if err != nil {
		_ = connection.WriteControl(gorilla.CloseMessage, gorilla.FormatCloseMessage(gorilla.CloseInternalServerErr, "unable to load transfers"), time.Now().Add(writeTimeout))
		return
	}
	now := time.Now().UTC()
	for _, item := range items {
		if err := writeJSON(connection, transfer.Event{Type: transfer.EventQueueUpdated, Transfer: item, Timestamp: now}); err != nil {
			return
		}
	}
	disconnected := make(chan struct{})
	go readPump(connection, disconnected)
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case <-disconnected:
			return
		case event, ok := <-events:
			if !ok || writeJSON(connection, event) != nil {
				return
			}
		case <-ping.C:
			if err := connection.WriteControl(gorilla.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				return
			}
		}
	}
}
