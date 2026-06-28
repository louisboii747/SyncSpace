package websocket

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
)

type staticTrustedDevices struct {
	devices []pairing.TrustedDevice
}

func (s staticTrustedDevices) TrustedDevices(context.Context) ([]pairing.TrustedDevice, error) {
	return s.devices, nil
}

func TestPairingHandlerReplaysTrustAndStreamsEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	broker := NewPairingBroker()
	handler := NewPairingHandler(
		broker,
		staticTrustedDevices{devices: []pairing.TrustedDevice{{DeviceID: "existing"}}},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	router := gin.New()
	router.GET("/ws/pairing", handler.Serve)
	server := httptest.NewServer(router)
	defer server.Close()

	address := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/pairing"
	connection, _, err := gorilla.DefaultDialer.Dial(address, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))

	var replay pairing.Event
	if err := connection.ReadJSON(&replay); err != nil {
		t.Fatal(err)
	}
	if replay.Type != pairing.EventPairingAccepted || replay.TrustedDevice == nil || replay.TrustedDevice.DeviceID != "existing" {
		t.Fatalf("unexpected pairing snapshot: %#v", replay)
	}

	request := pairing.Request{RequestID: "new-request"}
	broker.Publish(pairing.Event{Type: pairing.EventPairingRequested, Request: &request})
	var live pairing.Event
	if err := connection.ReadJSON(&live); err != nil {
		t.Fatal(err)
	}
	if live.Type != pairing.EventPairingRequested || live.Request == nil || live.Request.RequestID != "new-request" {
		t.Fatalf("unexpected live pairing event: %#v", live)
	}
}
