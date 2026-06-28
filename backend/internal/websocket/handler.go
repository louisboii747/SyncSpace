package websocket

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	"github.com/louisboii747/syncspace/backend/internal/models"
)

const (
	writeTimeout = 10 * time.Second
	pongTimeout  = 60 * time.Second
	pingInterval = 25 * time.Second
)

// DeviceLister provides the initial peer snapshot for a new socket.
type DeviceLister interface {
	Devices() []models.Device
}

// Handler upgrades discovery event connections and maintains their heartbeat.
type Handler struct {
	broker   *Broker
	devices  DeviceLister
	logger   *slog.Logger
	upgrader gorilla.Upgrader
}

// NewHandler constructs a discovery WebSocket handler.
func NewHandler(broker *Broker, devices DeviceLister, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
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

// Serve handles GET /ws/discovery. Existing peers are replayed as
// DeviceDiscovered events before live events, eliminating a polling race.
func (h *Handler) Serve(c *gin.Context) {
	connection, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Warn("WebSocket upgrade failed", "error", err, "remote_address", c.Request.RemoteAddr)
		return
	}
	defer connection.Close()

	events, unsubscribe := h.broker.Subscribe()
	defer unsubscribe()

	now := time.Now().UTC()
	for _, device := range h.devices.Devices() {
		if err := writeJSON(connection, models.DiscoveryEvent{
			Type:      models.EventDeviceDiscovered,
			Device:    device,
			Timestamp: now,
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

func readPump(connection *gorilla.Conn, disconnected chan<- struct{}) {
	defer close(disconnected)
	connection.SetReadLimit(1024)
	_ = connection.SetReadDeadline(time.Now().Add(pongTimeout))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(pongTimeout))
	})
	for {
		if _, _, err := connection.NextReader(); err != nil {
			return
		}
	}
}

func writeJSON(connection *gorilla.Conn, value any) error {
	if err := connection.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	return connection.WriteJSON(value)
}

func sameHostOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, request.Host)
}
