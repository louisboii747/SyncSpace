package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/settings"
)

func TestPrivacyGateBlocksNetworkFeaturesUntilCurrentPolicyIsAccepted(t *testing.T) {
	store, err := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	discovery := &fakeDiscoveryService{}
	router := NewRouter(RouterConfig{
		Discovery:       discovery,
		DiscoverySocket: func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) },
		Pairing:         &fakePairingService{},
		PairingSocket:   func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) },
		Settings:        store,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	self := versionedRequest(http.MethodGet, "/api/v1/device/self", nil)
	selfResponse := httptest.NewRecorder()
	router.ServeHTTP(selfResponse, self)
	if selfResponse.Code != http.StatusOK {
		t.Fatalf("self route was not available before acceptance: %d", selfResponse.Code)
	}

	blocked := versionedRequest(http.MethodPost, "/api/v1/discovery/refresh", nil)
	blockedResponse := httptest.NewRecorder()
	router.ServeHTTP(blockedResponse, blocked)
	if blockedResponse.Code != http.StatusForbidden || discovery.refreshes.Load() != 0 {
		t.Fatalf("discovery was not gated: status=%d refreshes=%d", blockedResponse.Code, discovery.refreshes.Load())
	}
	directDevices := versionedRequest(http.MethodGet, "/devices", nil)
	directDevicesResponse := httptest.NewRecorder()
	router.ServeHTTP(directDevicesResponse, directDevices)
	if directDevicesResponse.Code != http.StatusForbidden {
		t.Fatalf("unversioned discovery route was not gated: %d", directDevicesResponse.Code)
	}
	directSettings := versionedRequest(http.MethodPost, "/settings/reset", nil)
	directSettingsResponse := httptest.NewRecorder()
	router.ServeHTTP(directSettingsResponse, directSettings)
	if directSettingsResponse.Code != http.StatusForbidden {
		t.Fatalf("unversioned settings mutation was not gated: %d", directSettingsResponse.Code)
	}

	peer := versionedRequest(http.MethodPost, "/v1/pairing/requests", bytes.NewBufferString(`{}`))
	peerResponse := httptest.NewRecorder()
	router.ServeHTTP(peerResponse, peer)
	if peerResponse.Code != http.StatusForbidden {
		t.Fatalf("peer route was not gated: %d", peerResponse.Code)
	}

	body := bytes.NewBufferString(`{"version":"` + settings.CurrentPrivacyPolicyVersion + `"}`)
	accept := versionedRequest(http.MethodPost, "/api/v1/privacy-policy/accept", body)
	accept.Header.Set("Content-Type", "application/json")
	acceptResponse := httptest.NewRecorder()
	router.ServeHTTP(acceptResponse, accept)
	if acceptResponse.Code != http.StatusOK || !store.PrivacyAccepted() {
		t.Fatalf("privacy acceptance failed: status=%d body=%s", acceptResponse.Code, acceptResponse.Body.String())
	}

	allowed := versionedRequest(http.MethodPost, "/api/v1/discovery/refresh", nil)
	allowedResponse := httptest.NewRecorder()
	router.ServeHTTP(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusAccepted || discovery.refreshes.Load() != 1 {
		t.Fatalf("discovery did not unlock: status=%d refreshes=%d", allowedResponse.Code, discovery.refreshes.Load())
	}
}

func TestIncomingTransferPreferenceRejectsOnlyNewOffers(t *testing.T) {
	store, err := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AcceptPrivacy(settings.CurrentPrivacyPolicyVersion, time.Now()); err != nil {
		t.Fatal(err)
	}
	values := store.Get()
	values.IncomingTransfersEnabled = false
	if _, err := store.Update(values); err != nil {
		t.Fatal(err)
	}
	service := &fakeTransferService{}
	router := NewRouter(RouterConfig{
		Discovery: &fakeDiscoveryService{}, DiscoverySocket: func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) },
		Pairing: &fakePairingService{}, PairingSocket: func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) },
		Transfer: service, TransferSocket: func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) },
		Settings: store, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	offer := versionedRequest(http.MethodPost, "/v1/transfers/offers", bytes.NewBufferString(`{}`))
	offer.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, offer)
	if response.Code != http.StatusForbidden || service.offered.TransferID != "" {
		t.Fatalf("incoming offer preference was not enforced: status=%d offer=%#v", response.Code, service.offered)
	}
}

func versionedRequest(method, path string, body *bytes.Buffer) *http.Request {
	var reader io.Reader
	if body != nil {
		reader = body
	}
	request := httptest.NewRequest(method, path, reader)
	request.RemoteAddr = "127.0.0.1:5000"
	return request
}
