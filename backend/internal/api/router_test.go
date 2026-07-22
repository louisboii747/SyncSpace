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
func (s *fakePairingService) Requests() []pairing.Request { return nil }
func (s *fakePairingService) Accept(context.Context, string) (pairing.Decision, error) {
	return pairing.Decision{}, nil
}
func (s *fakePairingService) Refresh(context.Context, string) (pairing.Decision, error) {
	return pairing.Decision{}, nil
}
func (s *fakePairingService) Reject(context.Context, string) (pairing.Request, error) {
	return pairing.Request{}, nil
}
func (s *fakePairingService) ReceiveBegin(context.Context, pairing.BeginRequest, string) (pairing.BeginResponse, error) {
	return pairing.BeginResponse{}, nil
}
func (s *fakePairingService) ReceiveProof(context.Context, pairing.Proof) (pairing.PeerDecision, error) {
	return pairing.PeerDecision{}, nil
}
func (s *fakePairingService) RemoveTrustedDevice(context.Context, string) (pairing.TrustedDevice, error) {
	return pairing.TrustedDevice{}, nil
}
func (s *fakePairingService) SetBlocked(context.Context, string, bool) (pairing.TrustedDevice, error) {
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
	devicesRequest := httptest.NewRequest(http.MethodGet, "/devices", nil)
	devicesRequest.RemoteAddr = "127.0.0.1:54321"
	router.ServeHTTP(devicesResponse, devicesRequest)
	if devicesResponse.Code != http.StatusOK {
		t.Fatalf("GET /devices status = %d", devicesResponse.Code)
	}
	var devices []models.Device
	if err := json.Unmarshal(devicesResponse.Body.Bytes(), &devices); err != nil || len(devices) != 1 {
		t.Fatalf("GET /devices body = %s, error = %v", devicesResponse.Body, err)
	}

	selfResponse := httptest.NewRecorder()
	selfRequest := httptest.NewRequest(http.MethodGet, "/device/self", nil)
	selfRequest.RemoteAddr = "127.0.0.1:54321"
	router.ServeHTTP(selfResponse, selfRequest)
	if selfResponse.Code != http.StatusOK {
		t.Fatalf("GET /device/self status = %d", selfResponse.Code)
	}
	var self models.Device
	if err := json.Unmarshal(selfResponse.Body.Bytes(), &self); err != nil || self.ID != "self" {
		t.Fatalf("GET /device/self body = %s, error = %v", selfResponse.Body, err)
	}

	refreshResponse := httptest.NewRecorder()
	refreshRequest := httptest.NewRequest(http.MethodPost, "/discovery/refresh", nil)
	refreshRequest.RemoteAddr = "127.0.0.1:54321"
	router.ServeHTTP(refreshResponse, refreshRequest)
	if refreshResponse.Code != http.StatusAccepted || service.refreshes.Load() != 1 {
		t.Fatalf("POST /discovery/refresh status = %d, refreshes = %d", refreshResponse.Code, service.refreshes.Load())
	}
}

func TestVersionedManagementAliasAndCrossOriginProtection(t *testing.T) {
	service := &fakeDiscoveryService{devices: []models.Device{{ID: "peer"}}}
	router := NewRouter(RouterConfig{Discovery: service, DiscoverySocket: func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) }, Pairing: &fakePairingService{}, PairingSocket: func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	versioned := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	versioned.RemoteAddr = "127.0.0.1:54321"
	versionedResponse := httptest.NewRecorder()
	router.ServeHTTP(versionedResponse, versioned)
	if versionedResponse.Code != http.StatusOK {
		t.Fatalf("versioned devices status=%d body=%s", versionedResponse.Code, versionedResponse.Body)
	}

	crossSite := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/refresh", nil)
	crossSite.RemoteAddr = "127.0.0.1:54321"
	crossSite.Host = "127.0.0.1:8384"
	crossSite.Header.Set("Origin", "https://malicious.example")
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	crossSiteResponse := httptest.NewRecorder()
	router.ServeHTTP(crossSiteResponse, crossSite)
	if crossSiteResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation status=%d", crossSiteResponse.Code)
	}

	otherLocalPort := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/refresh", nil)
	otherLocalPort.RemoteAddr = "127.0.0.1:54321"
	otherLocalPort.Host = "127.0.0.1:8384"
	otherLocalPort.Header.Set("Origin", "http://127.0.0.1:5173")
	otherLocalPortResponse := httptest.NewRecorder()
	router.ServeHTTP(otherLocalPortResponse, otherLocalPort)
	if otherLocalPortResponse.Code != http.StatusForbidden {
		t.Fatalf("different localhost origin status=%d", otherLocalPortResponse.Code)
	}

	desktop := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/refresh", nil)
	desktop.RemoteAddr = "127.0.0.1:54321"
	desktop.Host = "127.0.0.1:8384"
	desktop.Header.Set("Origin", "http://wails.localhost")
	desktop.Header.Set("Sec-Fetch-Site", "cross-site")
	desktopResponse := httptest.NewRecorder()
	router.ServeHTTP(desktopResponse, desktop)
	if desktopResponse.Code != http.StatusAccepted || desktopResponse.Header().Get("Access-Control-Allow-Origin") != "http://wails.localhost" {
		t.Fatalf("Wails desktop request status=%d origin=%q", desktopResponse.Code, desktopResponse.Header().Get("Access-Control-Allow-Origin"))
	}

	missing := httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil)
	missing.RemoteAddr = "127.0.0.1:54321"
	missingResponse := httptest.NewRecorder()
	router.ServeHTTP(missingResponse, missing)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("unknown versioned route status=%d body=%s", missingResponse.Code, missingResponse.Body)
	}
}
