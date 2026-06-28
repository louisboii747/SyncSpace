package websocket

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	"github.com/louisboii747/syncspace/backend/internal/models"
)

type staticDevices struct {
	devices []models.Device
}

func (s staticDevices) Devices() []models.Device { return s.devices }

func TestHandlerReplaysSnapshotAndStreamsEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker := NewBroker()
	handler := NewHandler(
		broker,
		staticDevices{devices: []models.Device{{ID: "existing", Online: true}}},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	router := gin.New()
	router.GET("/ws/discovery", handler.Serve)
	server := httptest.NewServer(router)
	defer server.Close()

	address := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/discovery"
	connection, _, err := gorilla.DefaultDialer.Dial(address, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))

	var replay models.DiscoveryEvent
	if err := connection.ReadJSON(&replay); err != nil {
		t.Fatal(err)
	}
	if replay.Type != models.EventDeviceDiscovered || replay.Device.ID != "existing" {
		t.Fatalf("unexpected snapshot event: %#v", replay)
	}

	broker.Publish(models.DiscoveryEvent{Type: models.EventDeviceOffline, Device: models.Device{ID: "peer"}})
	var live models.DiscoveryEvent
	if err := connection.ReadJSON(&live); err != nil {
		t.Fatal(err)
	}
	if live.Type != models.EventDeviceOffline || live.Device.ID != "peer" {
		t.Fatalf("unexpected live event: %#v", live)
	}
}
