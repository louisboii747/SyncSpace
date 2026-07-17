package pairing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/services"
)

type memoryTrustedStore struct{ devices map[string]TrustedDevice }

func newMemoryTrustedStore() *memoryTrustedStore {
	return &memoryTrustedStore{devices: make(map[string]TrustedDevice)}
}
func (s *memoryTrustedStore) List(context.Context) ([]TrustedDevice, error) {
	result := make([]TrustedDevice, 0, len(s.devices))
	for _, device := range s.devices {
		result = append(result, device)
	}
	return result, nil
}
func (s *memoryTrustedStore) Get(_ context.Context, id string) (TrustedDevice, error) {
	device, ok := s.devices[id]
	if !ok {
		return TrustedDevice{}, ErrTrustedDeviceNotFound
	}
	return device, nil
}
func (s *memoryTrustedStore) Upsert(_ context.Context, device TrustedDevice) error {
	s.devices[device.DeviceID] = device
	return nil
}
func (s *memoryTrustedStore) Delete(_ context.Context, id string) (TrustedDevice, error) {
	device, ok := s.devices[id]
	if !ok {
		return TrustedDevice{}, ErrTrustedDeviceNotFound
	}
	delete(s.devices, id)
	return device, nil
}

type staticPeerDirectory struct{ devices []models.Device }

func (d *staticPeerDirectory) Devices() []models.Device {
	return append([]models.Device(nil), d.devices...)
}

type linkTransport struct {
	target     *Service
	remoteHost string
}

func (t *linkTransport) Begin(ctx context.Context, _ models.Device, request BeginRequest) (BeginResponse, error) {
	return t.target.ReceiveBegin(ctx, request, t.remoteHost)
}
func (t *linkTransport) Proof(ctx context.Context, _ models.Device, proof Proof) (PeerDecision, error) {
	return t.target.ReceiveProof(ctx, proof)
}

type eventRecorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *eventRecorder) Publish(event Event) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func testIdentity(t *testing.T, name, platform string) services.Identity {
	t.Helper()
	identity, err := services.NewFileIdentityStore(filepath.Join(t.TempDir(), name, "identity.json")).LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	identity.Name, identity.Platform = name, platform
	return identity
}

func peerFor(identity services.Identity, now time.Time) models.Device {
	return models.Device{ID: identity.ID, Name: identity.Name, Type: identity.Type, Platform: identity.Platform, LocalIP: "127.0.0.1", Port: 8385, AppVersion: "test", LastSeen: now, Online: true, ConnectionState: models.ConnectionOnline, IdentityHint: identity.ShortFingerprint(), PairingAvailable: true}
}

func pairedServices(t *testing.T, now func() time.Time) (*Service, *Service, *memoryTrustedStore, *memoryTrustedStore) {
	t.Helper()
	identityA, identityB := testIdentity(t, "Device A", "windows"), testIdentity(t, "Device B", "linux")
	storeA, storeB := newMemoryTrustedStore(), newMemoryTrustedStore()
	transportA, transportB := &linkTransport{remoteHost: "127.0.0.1"}, &linkTransport{remoteHost: "127.0.0.1"}
	newService := func(identity services.Identity, peer models.Device, store TrustedDeviceStore, transport PeerTransport) *Service {
		service, err := NewService(ServiceConfig{Store: store, Peers: &staticPeerDirectory{devices: []models.Device{peer}}, Identity: identity, Transport: transport, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: now})
		if err != nil {
			t.Fatal(err)
		}
		return service
	}
	a := newService(identityA, peerFor(identityB, now()), storeA, transportA)
	b := newService(identityB, peerFor(identityA, now()), storeB, transportB)
	transportA.target, transportB.target = b, a
	return a, b, storeA, storeB
}

func TestAuthenticatedPairingRequiresMatchingCodesAndBothConfirmations(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	a, b, storeA, storeB := pairedServices(t, func() time.Time { return now })
	peerID := b.identity.ID
	requestA, err := a.RequestPairing(ctx, peerID)
	if err != nil {
		t.Fatal(err)
	}
	requestsB := b.Requests()
	if len(requestsB) != 1 || requestsB[0].VerificationCode != requestA.VerificationCode || requestsB[0].Direction != DirectionIncoming {
		t.Fatalf("verification ceremony did not match: A=%#v B=%#v", requestA, requestsB)
	}
	if len(storeA.devices) != 0 || len(storeB.devices) != 0 {
		t.Fatal("key exchange created trust before user confirmation")
	}

	decisionA, err := a.Accept(ctx, requestA.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if decisionA.TrustedDevice != nil || decisionA.Request.State != RequestStateConfirming {
		t.Fatalf("first confirmation trusted prematurely: %#v", decisionA)
	}
	decisionB, err := b.Accept(ctx, requestA.RequestID)
	if err != nil || decisionB.TrustedDevice == nil {
		t.Fatalf("receiver confirmation did not complete local trust: %#v %v", decisionB, err)
	}
	decisionA, err = a.Refresh(ctx, requestA.RequestID)
	if err != nil || decisionA.TrustedDevice == nil {
		t.Fatalf("initiator did not observe peer confirmation: %#v %v", decisionA, err)
	}
	trustedA, trustedB := storeA.devices[b.identity.ID], storeB.devices[a.identity.ID]
	if trustedA.PairingKey == "" || trustedA.PairingKey != trustedB.PairingKey || trustedA.PublicKey != b.identity.PublicKey || trustedB.PublicKey != a.identity.PublicKey {
		t.Fatalf("paired credentials differ: A=%#v B=%#v", trustedA, trustedB)
	}
	encoded, _ := json.Marshal(trustedA)
	if string(encoded) == "" || contains(string(encoded), trustedA.PairingKey) {
		t.Fatal("trusted-device API leaked the shared pairing key")
	}
}

func TestPairingRejectsDiscoveryIdentityMismatchAndReplayedProof(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	a, b, _, _ := pairedServices(t, func() time.Time { return now })
	a.peers.(*staticPeerDirectory).devices[0].IdentityHint = "FFFF:FFFF:FFFF:FFFF"
	if _, err := a.RequestPairing(ctx, b.identity.ID); !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected discovery identity mismatch, got %v", err)
	}
	a, b, _, _ = pairedServices(t, func() time.Time { return now })
	request, err := a.RequestPairing(ctx, b.identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	session := a.sessions[request.RequestID]
	proof := newProof(b.identity.ID, request.RequestID, "status", session.secret, now)
	if _, err := a.ReceiveProof(ctx, proof); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ReceiveProof(ctx, proof); !errors.Is(err, ErrProtocol) {
		t.Fatalf("replayed proof was accepted: %v", err)
	}
}

func TestPairingRejectBlockAndExpiryLifecycle(t *testing.T) {
	ctx := context.Background()
	current := time.Now().UTC()
	a, b, _, _ := pairedServices(t, func() time.Time { return current })
	request, err := a.RequestPairing(ctx, b.identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Reject(ctx, request.RequestID); err != nil {
		t.Fatal(err)
	}
	if decision, err := a.Accept(ctx, request.RequestID); err != nil || decision.Request.State != RequestStateRejected {
		t.Fatalf("rejected request changed state: %#v %v", decision, err)
	}
	current = current.Add(6 * time.Minute)
	if _, err := a.Refresh(ctx, request.RequestID); !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expired request remained available: %v", err)
	}
}

func contains(value, fragment string) bool {
	return len(fragment) > 0 && len(value) >= len(fragment) && func() bool {
		for index := 0; index+len(fragment) <= len(value); index++ {
			if value[index:index+len(fragment)] == fragment {
				return true
			}
		}
		return false
	}()
}
