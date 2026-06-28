package websocket

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
)

// TrustedDeviceLister supplies the snapshot replayed to new pairing sockets.
type TrustedDeviceLister interface {
	TrustedDevices(context.Context) ([]pairing.TrustedDevice, error)
}

// PairingHandler upgrades and maintains pairing event connections.
type PairingHandler struct {
	broker   *PairingBroker
	devices  TrustedDeviceLister
	logger   *slog.Logger
	upgrader gorilla.Upgrader
}

// NewPairingHandler constructs a pairing WebSocket handler.
func NewPairingHandler(broker *PairingBroker, devices TrustedDeviceLister, logger *slog.Logger) *PairingHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PairingHandler{
		broker:  broker,
		devices: devices,
		logger:  logger,
		upgrader: gorilla.Upgrader{
			HandshakeTimeout: 10 * time.Second,
			ReadBufferSize:   1024,
			WriteBufferSize:  4096,
			CheckOrigin:      sameHostOrigin,
		},
	}
}

// Serve handles GET /ws/pairing and replays current trust as PairingAccepted
// events before switching to live transitions.
func (h *PairingHandler) Serve(c *gin.Context) {
	connection, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Warn("Pairing WebSocket upgrade failed", "error", err, "remote_address", c.Request.RemoteAddr)
		return
	}
	defer connection.Close()

	events, unsubscribe := h.broker.Subscribe()
	defer unsubscribe()
	trustedDevices, err := h.devices.TrustedDevices(c.Request.Context())
	if err != nil {
		h.logger.Error("Pairing WebSocket snapshot failed", "error", err)
		_ = connection.WriteControl(gorilla.CloseMessage,
			gorilla.FormatCloseMessage(gorilla.CloseInternalServerErr, "unable to load trusted devices"),
			time.Now().Add(writeTimeout),
		)
		return
	}
	now := time.Now().UTC()
	for index := range trustedDevices {
		device := trustedDevices[index]
		if err := writeJSON(connection, pairing.Event{
			Type:          pairing.EventPairingAccepted,
			TrustedDevice: &device,
			Timestamp:     now,
		}); err != nil {
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
