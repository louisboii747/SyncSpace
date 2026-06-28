package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
)

type fakeDiscoveryService struct {
	devices   []models.Device
	self      models.Device
	refreshes atomic.Int32
}

func (s *fakeDiscoveryService) Devices() []models.Device { return s.devices }
func (s *fakeDiscoveryService) Self() models.Device      { return s.self }
func (s *fakeDiscoveryService) Refresh()                 { s.refreshes.Add(1) }

type fakePairingService struct{}

func (s *fakePairingService) TrustedDevices(context.Context) ([]pairing.TrustedDevice, error) {
	return []pairing.TrustedDevice{}, nil
}
func (s *fakePairingService) RequestPairing(context.Context, string) (pairing.Request, error) {
	return pairing.Request{}, nil
}
func (s *fakePairingService) Accept(context.Context, string) (pairing.TrustedDevice, error) {
	return pairing.TrustedDevice{}, nil
}
func (s *fakePairingService) Reject(string) (pairing.Request, error) {
	return pairing.Request{}, nil
}
func (s *fakePairingService) RemoveTrustedDevice(context.Context, string) (pairing.TrustedDevice, error) {
	return pairing.TrustedDevice{}, nil
}

func TestDiscoveryRoutes(t *testing.T) {
	service := &fakeDiscoveryService{
		devices: []models.Device{{ID: "peer"}},
		self:    models.Device{ID: "self"},
	}
	router := NewRouter(RouterConfig{
		Discovery: service,
		DiscoverySocket: func(c *gin.Context) {
			c.Status(http.StatusSwitchingProtocols)
		},
		Pairing: &fakePairingService{},
		PairingSocket: func(c *gin.Context) {
			c.Status(http.StatusSwitchingProtocols)
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	devicesResponse := httptest.NewRecorder()
	router.ServeHTTP(devicesResponse, httptest.NewRequest(http.MethodGet, "/devices", nil))
	if devicesResponse.Code != http.StatusOK {
		t.Fatalf("GET /devices status = %d", devicesResponse.Code)
	}
	var devices []models.Device
	if err := json.Unmarshal(devicesResponse.Body.Bytes(), &devices); err != nil || len(devices) != 1 {
		t.Fatalf("GET /devices body = %s, error = %v", devicesResponse.Body, err)
	}

	selfResponse := httptest.NewRecorder()
	router.ServeHTTP(selfResponse, httptest.NewRequest(http.MethodGet, "/device/self", nil))
	if selfResponse.Code != http.StatusOK {
		t.Fatalf("GET /device/self status = %d", selfResponse.Code)
	}
	var self models.Device
	if err := json.Unmarshal(selfResponse.Body.Bytes(), &self); err != nil || self.ID != "self" {
		t.Fatalf("GET /device/self body = %s, error = %v", selfResponse.Body, err)
	}

	refreshResponse := httptest.NewRecorder()
	router.ServeHTTP(refreshResponse, httptest.NewRequest(http.MethodPost, "/discovery/refresh", nil))
	if refreshResponse.Code != http.StatusAccepted || service.refreshes.Load() != 1 {
		t.Fatalf("POST /discovery/refresh status = %d, refreshes = %d", refreshResponse.Code, service.refreshes.Load())
	}
}
