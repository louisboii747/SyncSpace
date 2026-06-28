package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
)

type recordingPairingService struct {
	trusted     []pairing.TrustedDevice
	requestedID string
	acceptedID  string
	rejectedID  string
	removedID   string
}

func (s *recordingPairingService) TrustedDevices(context.Context) ([]pairing.TrustedDevice, error) {
	return s.trusted, nil
}
func (s *recordingPairingService) RequestPairing(_ context.Context, id string) (pairing.Request, error) {
	s.requestedID = id
	return pairing.Request{DeviceID: id}, nil
}
func (s *recordingPairingService) Accept(_ context.Context, id string) (pairing.TrustedDevice, error) {
	s.acceptedID = id
	return pairing.TrustedDevice{DeviceID: "accepted"}, nil
}
func (s *recordingPairingService) Reject(id string) (pairing.Request, error) {
	s.rejectedID = id
	return pairing.Request{RequestID: id}, nil
}
func (s *recordingPairingService) RemoveTrustedDevice(_ context.Context, id string) (pairing.TrustedDevice, error) {
	s.removedID = id
	return pairing.TrustedDevice{DeviceID: id}, nil
}

func TestPairingRoutes(t *testing.T) {
	service := &recordingPairingService{trusted: []pairing.TrustedDevice{{DeviceID: "trusted"}}}
	router := gin.New()
	registerPairingRoutes(router, service, func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) }, slog.New(slog.NewTextHandler(io.Discard, nil)))

	assertStatus(t, router, http.MethodGet, "/pairing/trusted-devices", "", http.StatusOK)
	deviceID := uuid.NewString()
	assertStatus(t, router, http.MethodPost, "/pairing/request", `{"deviceId":"`+deviceID+`"}`, http.StatusAccepted)
	requestID := uuid.NewString()
	assertStatus(t, router, http.MethodPost, "/pairing/accept", `{"requestId":"`+requestID+`"}`, http.StatusOK)
	rejectedID := uuid.NewString()
	assertStatus(t, router, http.MethodPost, "/pairing/reject", `{"requestId":"`+rejectedID+`"}`, http.StatusOK)
	removedID := uuid.NewString()
	assertStatus(t, router, http.MethodDelete, "/pairing/trusted-devices/"+removedID, "", http.StatusNoContent)

	if service.requestedID != deviceID || service.acceptedID != requestID ||
		service.rejectedID != rejectedID || service.removedID != removedID {
		t.Fatalf("route values were not forwarded: %#v", service)
	}
}

func TestPairingRouteRejectsMalformedRequest(t *testing.T) {
	router := gin.New()
	registerPairingRoutes(router, &recordingPairingService{}, func(c *gin.Context) {}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	assertStatus(t, router, http.MethodPost, "/pairing/request", `{}`, http.StatusBadRequest)
}

func TestPairingRoutesRejectRemoteManagement(t *testing.T) {
	router := gin.New()
	registerPairingRoutes(router, &recordingPairingService{}, func(c *gin.Context) {}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/pairing/trusted-devices", nil)
	request.RemoteAddr = "192.168.1.55:54321"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("remote pairing management status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func assertStatus(t *testing.T, handler http.Handler, method, path, body string, expected int) {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:54321"
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != expected {
		t.Fatalf("%s %s status = %d, want %d; body = %s", method, path, response.Code, expected, response.Body)
	}
}
