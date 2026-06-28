package discovery

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/models"
)

type recordingPublisher struct {
	mu     sync.Mutex
	events []models.DiscoveryEvent
}

func (p *recordingPublisher) Publish(event models.DiscoveryEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *recordingPublisher) types() []models.DiscoveryEventType {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]models.DiscoveryEventType, 0, len(p.events))
	for _, event := range p.events {
		result = append(result, event.Type)
	}
	return result
}

func TestRegistryLifecycleAndDuplicateBroadcasts(t *testing.T) {
	publisher := &recordingPublisher{}
	registry, err := NewRegistry(RegistryConfig{
		SelfID:       uuid.NewString(),
		OfflineAfter: 30 * time.Second,
		RemoveAfter:  2 * time.Minute,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Publisher:    publisher,
	})
	if err != nil {
		t.Fatal(err)
	}

	device := testDevice()
	firstSeen := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	if err := registry.Upsert(device, firstSeen); err != nil {
		t.Fatal(err)
	}
	if err := registry.Upsert(device, firstSeen.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	assertEventTypes(t, publisher.types(), models.EventDeviceDiscovered)

	devices := registry.List()
	if len(devices) != 1 || !devices[0].LastSeen.Equal(firstSeen.Add(5*time.Second)) {
		t.Fatalf("duplicate broadcast did not refresh last seen: %#v", devices)
	}

	device.AppVersion = "1.1.0"
	if err := registry.Upsert(device, firstSeen.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	registry.Sweep(firstSeen.Add(41 * time.Second))
	registry.Sweep(firstSeen.Add(131 * time.Second))
	assertEventTypes(t, publisher.types(),
		models.EventDeviceDiscovered,
		models.EventDeviceUpdated,
		models.EventDeviceOffline,
		models.EventDeviceRemoved,
	)
	if devices := registry.List(); len(devices) != 0 {
		t.Fatalf("expected expired device removal, got %#v", devices)
	}
}

func TestRegistryRejectsConflictingIdentity(t *testing.T) {
	registry, err := NewRegistry(RegistryConfig{
		SelfID:       uuid.NewString(),
		OfflineAfter: time.Second,
		RemoveAfter:  2 * time.Second,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	device := testDevice()
	if err := registry.Upsert(device, time.Now()); err != nil {
		t.Fatal(err)
	}
	device.Name = "Cloned Identity"
	if err := registry.Upsert(device, time.Now()); !errors.Is(err, ErrDuplicateIdentity) {
		t.Fatalf("expected duplicate identity error, got %v", err)
	}
}

func testDevice() models.Device {
	return models.Device{
		ID:         uuid.NewString(),
		Name:       "Test-PC",
		Type:       "desktop",
		Platform:   "windows",
		LocalIP:    "192.168.1.12",
		Port:       8384,
		AppVersion: "1.0.0",
	}
}

func assertEventTypes(t *testing.T, actual []models.DiscoveryEventType, expected ...models.DiscoveryEventType) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("event count: got %v, want %v", actual, expected)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("event %d: got %s, want %s", index, actual[index], expected[index])
		}
	}
}
