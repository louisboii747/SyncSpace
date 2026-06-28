package pairing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/models"
)

type memoryTrustedStore struct {
	devices map[string]TrustedDevice
}

func newMemoryTrustedStore() *memoryTrustedStore {
	return &memoryTrustedStore{devices: make(map[string]TrustedDevice)}
}

func (s *memoryTrustedStore) List(context.Context) ([]TrustedDevice, error) {
	devices := make([]TrustedDevice, 0, len(s.devices))
	for _, device := range s.devices {
		devices = append(devices, device)
	}
	return devices, nil
}

func (s *memoryTrustedStore) Get(_ context.Context, id string) (TrustedDevice, error) {
	device, found := s.devices[id]
	if !found {
		return TrustedDevice{}, ErrTrustedDeviceNotFound
	}
	return device, nil
}

func (s *memoryTrustedStore) Upsert(_ context.Context, device TrustedDevice) error {
	s.devices[device.DeviceID] = device
	return nil
}

func (s *memoryTrustedStore) Delete(_ context.Context, id string) (TrustedDevice, error) {
	device, found := s.devices[id]
	if !found {
		return TrustedDevice{}, ErrTrustedDeviceNotFound
	}
	delete(s.devices, id)
	return device, nil
}

type staticPeerDirectory struct {
	devices []models.Device
}

func (d *staticPeerDirectory) Devices() []models.Device { return d.devices }

type eventRecorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *eventRecorder) Publish(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *eventRecorder) types() []EventType {
	r.mu.Lock()
	defer r.mu.Unlock()
	types := make([]EventType, 0, len(r.events))
	for _, event := range r.events {
		types = append(types, event.Type)
	}
	return types
}

func TestServiceRequiresExplicitAcceptanceAndPersistsTrust(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	peer := models.Device{
		ID:       uuid.NewString(),
		Name:     "Peer-Mac",
		Platform: "darwin",
		LastSeen: now.Add(-time.Second),
		Online:   true,
	}
	store := newMemoryTrustedStore()
	directory := &staticPeerDirectory{devices: []models.Device{peer}}
	events := &eventRecorder{}
	service := newTestService(t, store, directory, events, func() time.Time { return now })

	trusted, err := service.TrustedDevices(ctx)
	if err != nil || len(trusted) != 0 {
		t.Fatalf("discovery automatically created trust: %#v, %v", trusted, err)
	}
	request, err := service.RequestPairing(ctx, peer.ID)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := service.RequestPairing(ctx, peer.ID)
	if err != nil || duplicate.RequestID != request.RequestID {
		t.Fatalf("duplicate request was not idempotent: %#v, %v", duplicate, err)
	}
	trusted, err = service.TrustedDevices(ctx)
	if err != nil || len(trusted) != 0 {
		t.Fatalf("pending request created trust: %#v, %v", trusted, err)
	}

	accepted, err := service.Accept(ctx, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.DeviceID != peer.ID || accepted.TrustState != TrustStateTrusted || accepted.PairingKey != "test-placeholder-key" {
		t.Fatalf("unexpected accepted trust: %#v", accepted)
	}
	restarted := newTestService(t, store, directory, nil, func() time.Time { return now.Add(time.Minute) })
	trusted, err = restarted.TrustedDevices(ctx)
	if err != nil || len(trusted) != 1 || trusted[0].DeviceID != peer.ID {
		t.Fatalf("trust did not survive service restart: %#v, %v", trusted, err)
	}
	assertPairingEventTypes(t, events.types(), EventPairingRequested, EventPairingAccepted)
}

func TestServiceRejectsAndRemovesTrust(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	peer := models.Device{ID: uuid.NewString(), Name: "Phone", Platform: "android", LastSeen: now, Online: true}
	store := newMemoryTrustedStore()
	directory := &staticPeerDirectory{devices: []models.Device{peer}}
	events := &eventRecorder{}
	service := newTestService(t, store, directory, events, func() time.Time { return now })

	request, err := service.RequestPairing(ctx, peer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reject(request.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Accept(ctx, request.RequestID); !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("rejected request could still be accepted: %v", err)
	}

	request, err = service.RequestPairing(ctx, peer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Accept(ctx, request.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RemoveTrustedDevice(ctx, peer.ID); err != nil {
		t.Fatal(err)
	}
	if devices, err := service.TrustedDevices(ctx); err != nil || len(devices) != 0 {
		t.Fatalf("trust was not removed: %#v, %v", devices, err)
	}
	assertPairingEventTypes(t, events.types(),
		EventPairingRequested,
		EventPairingRejected,
		EventPairingRequested,
		EventPairingAccepted,
		EventTrustedDeviceRemoved,
	)
}

func TestServiceExpiresPendingRequests(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	peer := models.Device{ID: uuid.NewString(), Name: "Phone", Platform: "ios", LastSeen: now, Online: true}
	service := newTestService(t, newMemoryTrustedStore(), &staticPeerDirectory{devices: []models.Device{peer}}, nil, func() time.Time { return now })
	request, err := service.RequestPairing(ctx, peer.ID)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now.Add(6 * time.Minute) }
	if _, err := service.Accept(ctx, request.RequestID); !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected expired request to be unavailable, got %v", err)
	}
}

func newTestService(t *testing.T, store TrustedDeviceStore, peers PeerDirectory, events EventPublisher, now func() time.Time) *Service {
	t.Helper()
	service, err := NewService(ServiceConfig{
		Store:       store,
		Peers:       peers,
		Publisher:   events,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:         now,
		GenerateKey: func() (string, error) { return "test-placeholder-key", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func assertPairingEventTypes(t *testing.T, actual []EventType, expected ...EventType) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("events = %v, want %v", actual, expected)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("event %d = %s, want %s", index, actual[index], expected[index])
		}
	}
}
