// Package discovery implements LAN discovery and the reusable in-memory peer
// registry.
package discovery

import (
	"errors"
	"log/slog"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/models"
)

var (
	// ErrInvalidDevice indicates that an advertisement is missing required or
	// trustworthy device metadata.
	ErrInvalidDevice = errors.New("invalid discovered device")
	// ErrDuplicateIdentity indicates that two advertisements claim the same ID
	// while presenting incompatible permanent identity metadata.
	ErrDuplicateIdentity = errors.New("duplicate device identity")
)

// EventPublisher receives registry lifecycle events.
type EventPublisher interface {
	Publish(models.DiscoveryEvent)
}

// RegistryConfig controls peer liveness transitions.
type RegistryConfig struct {
	SelfID       string
	OfflineAfter time.Duration
	RemoveAfter  time.Duration
	Logger       *slog.Logger
	Publisher    EventPublisher
}

// Registry is a concurrency-safe, transport-neutral store of discovered peers.
type Registry struct {
	mu           sync.RWMutex
	devices      map[string]models.Device
	selfID       string
	offlineAfter time.Duration
	removeAfter  time.Duration
	logger       *slog.Logger
	publisher    EventPublisher
}

// NewRegistry constructs an empty peer registry.
func NewRegistry(config RegistryConfig) (*Registry, error) {
	if config.OfflineAfter <= 0 || config.RemoveAfter <= config.OfflineAfter {
		return nil, errors.New("remove timeout must be greater than a positive offline timeout")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &Registry{
		devices:      make(map[string]models.Device),
		selfID:       config.SelfID,
		offlineAfter: config.OfflineAfter,
		removeAfter:  config.RemoveAfter,
		logger:       config.Logger,
		publisher:    config.Publisher,
	}, nil
}

// Upsert records an observed peer. Repeated mDNS packets only refresh
// LastSeen; events are emitted solely for meaningful state or metadata changes.
func (r *Registry) Upsert(device models.Device, observedAt time.Time) error {
	if err := validateDiscoveredDevice(device); err != nil {
		return err
	}
	if device.ID == r.selfID {
		return nil
	}

	device.LastSeen = observedAt.UTC()
	device.Online = true
	device.ConnectionState = models.ConnectionOnline

	var event *models.DiscoveryEvent
	r.mu.Lock()
	existing, found := r.devices[device.ID]
	if !found {
		r.devices[device.ID] = device
		event = newEvent(models.EventDeviceDiscovered, device, observedAt)
	} else {
		if identityConflicts(existing, device) {
			r.mu.Unlock()
			return ErrDuplicateIdentity
		}
		changed := !existing.Online || metadataChanged(existing, device)
		r.devices[device.ID] = device
		if changed {
			event = newEvent(models.EventDeviceUpdated, device, observedAt)
		}
	}
	r.mu.Unlock()

	if event != nil {
		r.logAndPublish(*event)
	}
	return nil
}

// List returns a stable snapshot sorted by friendly name and ID.
func (r *Registry) List() []models.Device {
	r.mu.RLock()
	devices := make([]models.Device, 0, len(r.devices))
	for _, device := range r.devices {
		devices = append(devices, device)
	}
	r.mu.RUnlock()

	sort.Slice(devices, func(i, j int) bool {
		left := strings.ToLower(devices[i].Name)
		right := strings.ToLower(devices[j].Name)
		if left == right {
			return devices[i].ID < devices[j].ID
		}
		return left < right
	})
	return devices
}

// Sweep marks stale peers offline and removes peers that have remained stale
// beyond the removal timeout.
func (r *Registry) Sweep(now time.Time) {
	now = now.UTC()
	events := make([]models.DiscoveryEvent, 0)

	r.mu.Lock()
	for id, device := range r.devices {
		age := now.Sub(device.LastSeen)
		if age >= r.removeAfter {
			delete(r.devices, id)
			device.Online = false
			device.ConnectionState = models.ConnectionOffline
			events = append(events, *newEvent(models.EventDeviceRemoved, device, now))
			continue
		}
		if device.Online && age >= r.offlineAfter {
			device.Online = false
			device.ConnectionState = models.ConnectionOffline
			r.devices[id] = device
			events = append(events, *newEvent(models.EventDeviceOffline, device, now))
		}
	}
	r.mu.Unlock()

	for _, event := range events {
		r.logAndPublish(event)
	}
}

func (r *Registry) logAndPublish(event models.DiscoveryEvent) {
	message := map[models.DiscoveryEventType]string{
		models.EventDeviceDiscovered: "Device found",
		models.EventDeviceUpdated:    "Device updated",
		models.EventDeviceOffline:    "Device expired",
		models.EventDeviceRemoved:    "Device removed",
	}[event.Type]
	r.logger.Info(message,
		"event", event.Type,
		"device_id", event.Device.ID,
		"device_name", event.Device.Name,
		"ip", event.Device.LocalIP,
		"port", event.Device.Port,
	)
	if r.publisher != nil {
		r.publisher.Publish(event)
	}
}

func validateDiscoveredDevice(device models.Device) error {
	if _, err := uuid.Parse(device.ID); err != nil {
		return errors.Join(ErrInvalidDevice, err)
	}
	if strings.TrimSpace(device.Name) == "" || len(device.Name) > 128 ||
		strings.TrimSpace(device.Type) == "" || len(device.Type) > 32 ||
		strings.TrimSpace(device.Platform) == "" || len(device.Platform) > 32 ||
		strings.TrimSpace(device.AppVersion) == "" || len(device.AppVersion) > 64 ||
		net.ParseIP(device.LocalIP) == nil || device.Port < 1 || device.Port > 65535 {
		return ErrInvalidDevice
	}
	return nil
}

func identityConflicts(existing, incoming models.Device) bool {
	return existing.Name != incoming.Name || existing.Type != incoming.Type || existing.Platform != incoming.Platform
}

func metadataChanged(existing, incoming models.Device) bool {
	return existing.Name != incoming.Name ||
		existing.Type != incoming.Type ||
		existing.Platform != incoming.Platform ||
		existing.LocalIP != incoming.LocalIP ||
		existing.Port != incoming.Port ||
		existing.AppVersion != incoming.AppVersion
}

func newEvent(eventType models.DiscoveryEventType, device models.Device, timestamp time.Time) *models.DiscoveryEvent {
	return &models.DiscoveryEvent{Type: eventType, Device: device, Timestamp: timestamp.UTC()}
}
