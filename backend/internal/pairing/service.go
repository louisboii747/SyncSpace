package pairing

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/models"
)

var (
	// ErrPeerNotDiscovered means a pairing request did not reference a currently
	// online discovered device.
	ErrPeerNotDiscovered = errors.New("device is not currently discovered")
	// ErrAlreadyTrusted means the device already has explicit local trust.
	ErrAlreadyTrusted = errors.New("device is already trusted")
	// ErrRequestNotFound means a pairing request is unknown or has expired.
	ErrRequestNotFound = errors.New("pairing request not found")
	// ErrInvalidIdentifier means a device or request ID is not a UUID.
	ErrInvalidIdentifier = errors.New("identifier must be a UUID")
)

// PeerDirectory is the narrow discovery contract needed by pairing.
type PeerDirectory interface {
	Devices() []models.Device
}

// KeyGenerator creates the placeholder pairing key stored with accepted trust.
// A future authenticated key-exchange implementation can replace this
// dependency without changing the service API or trusted-device store.
type KeyGenerator func() (string, error)

// ServiceConfig supplies pairing dependencies and lifecycle policy.
type ServiceConfig struct {
	Store       TrustedDeviceStore
	Peers       PeerDirectory
	Publisher   EventPublisher
	Logger      *slog.Logger
	RequestTTL  time.Duration
	Now         func() time.Time
	GenerateKey KeyGenerator
}

// Service owns pending pairing decisions and durable trusted devices.
type Service struct {
	mu              sync.Mutex
	requests        map[string]Request
	pendingByDevice map[string]string
	store           TrustedDeviceStore
	peers           PeerDirectory
	publisher       EventPublisher
	logger          *slog.Logger
	requestTTL      time.Duration
	now             func() time.Time
	generateKey     KeyGenerator
}

// NewService constructs a pairing service. It never imports discovered peers
// into trust automatically.
func NewService(config ServiceConfig) (*Service, error) {
	if config.Store == nil || config.Peers == nil {
		return nil, errors.New("trusted device store and peer directory are required")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.RequestTTL <= 0 {
		config.RequestTTL = 5 * time.Minute
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.GenerateKey == nil {
		config.GenerateKey = generatePlaceholderKey
	}
	return &Service{
		requests:        make(map[string]Request),
		pendingByDevice: make(map[string]string),
		store:           config.Store,
		peers:           config.Peers,
		publisher:       config.Publisher,
		logger:          config.Logger,
		requestTTL:      config.RequestTTL,
		now:             config.Now,
		generateKey:     config.GenerateKey,
	}, nil
}

// TrustedDevices returns durable trust records and refreshes LastSeen from the
// discovery directory without changing trust state.
func (s *Service) TrustedDevices(ctx context.Context) ([]TrustedDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	devices, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	peers := make(map[string]models.Device)
	for _, peer := range s.peers.Devices() {
		peers[peer.ID] = peer
	}
	for index := range devices {
		peer, found := peers[devices[index].DeviceID]
		if !found || !peer.LastSeen.After(devices[index].LastSeen) {
			continue
		}
		devices[index].LastSeen = peer.LastSeen.UTC()
		if err := s.store.Upsert(ctx, devices[index]); err != nil {
			return nil, err
		}
	}
	return devices, nil
}

// RequestPairing creates a short-lived request for an online discovered peer.
// Repeated requests for the same peer return the existing pending request.
func (s *Service) RequestPairing(ctx context.Context, deviceID string) (Request, error) {
	if err := validateUUID(deviceID); err != nil {
		return Request{}, err
	}
	peer, found := s.findOnlinePeer(deviceID)
	if !found {
		return Request{}, ErrPeerNotDiscovered
	}

	s.mu.Lock()
	now := s.now().UTC()
	s.pruneExpiredLocked(now)
	if _, err := s.store.Get(ctx, deviceID); err == nil {
		s.mu.Unlock()
		return Request{}, ErrAlreadyTrusted
	} else if !errors.Is(err, ErrTrustedDeviceNotFound) {
		s.mu.Unlock()
		return Request{}, err
	}
	if requestID, exists := s.pendingByDevice[deviceID]; exists {
		request := s.requests[requestID]
		s.mu.Unlock()
		return request, nil
	}
	pairingKey, err := s.generateKey()
	if err != nil {
		s.mu.Unlock()
		return Request{}, fmt.Errorf("generate pairing key placeholder: %w", err)
	}
	request := Request{
		RequestID:   uuid.NewString(),
		DeviceID:    peer.ID,
		DeviceName:  peer.Name,
		Platform:    peer.Platform,
		PairingKey:  pairingKey,
		RequestedAt: now,
		ExpiresAt:   now.Add(s.requestTTL),
		State:       RequestStatePending,
	}
	s.requests[request.RequestID] = request
	s.pendingByDevice[deviceID] = request.RequestID
	s.mu.Unlock()

	s.logger.Info("Pairing requested", "request_id", request.RequestID, "device_id", request.DeviceID)
	s.publish(Event{Type: EventPairingRequested, Request: &request, Timestamp: now})
	return request, nil
}

// Accept explicitly trusts the device in a pending request and commits that
// decision before publishing PairingAccepted.
func (s *Service) Accept(ctx context.Context, requestID string) (TrustedDevice, error) {
	if err := validateUUID(requestID); err != nil {
		return TrustedDevice{}, err
	}
	s.mu.Lock()
	now := s.now().UTC()
	s.pruneExpiredLocked(now)
	request, found := s.requests[requestID]
	if !found {
		s.mu.Unlock()
		return TrustedDevice{}, ErrRequestNotFound
	}
	lastSeen := request.RequestedAt
	if peer, found := s.findOnlinePeer(request.DeviceID); found && peer.LastSeen.After(lastSeen) {
		lastSeen = peer.LastSeen
	}
	trusted := TrustedDevice{
		DeviceID:   request.DeviceID,
		DeviceName: request.DeviceName,
		Platform:   request.Platform,
		PairingKey: request.PairingKey,
		PairedAt:   now,
		LastSeen:   lastSeen.UTC(),
		TrustState: TrustStateTrusted,
	}
	if err := validateTrustedDevice(trusted); err != nil {
		s.mu.Unlock()
		return TrustedDevice{}, err
	}
	if err := s.store.Upsert(ctx, trusted); err != nil {
		s.mu.Unlock()
		return TrustedDevice{}, err
	}
	delete(s.requests, requestID)
	delete(s.pendingByDevice, request.DeviceID)
	s.mu.Unlock()

	s.logger.Info("Pairing accepted", "request_id", requestID, "device_id", trusted.DeviceID)
	s.publish(Event{Type: EventPairingAccepted, TrustedDevice: &trusted, Timestamp: now})
	return trusted, nil
}

// Reject removes a pending request without creating trust.
func (s *Service) Reject(requestID string) (Request, error) {
	if err := validateUUID(requestID); err != nil {
		return Request{}, err
	}
	s.mu.Lock()
	now := s.now().UTC()
	s.pruneExpiredLocked(now)
	request, found := s.requests[requestID]
	if !found {
		s.mu.Unlock()
		return Request{}, ErrRequestNotFound
	}
	delete(s.requests, requestID)
	delete(s.pendingByDevice, request.DeviceID)
	request.State = RequestStateRejected
	s.mu.Unlock()

	s.logger.Info("Pairing rejected", "request_id", requestID, "device_id", request.DeviceID)
	s.publish(Event{Type: EventPairingRejected, Request: &request, Timestamp: now})
	return request, nil
}

// RemoveTrustedDevice revokes and deletes local trust for a device.
func (s *Service) RemoveTrustedDevice(ctx context.Context, deviceID string) (TrustedDevice, error) {
	if err := validateUUID(deviceID); err != nil {
		return TrustedDevice{}, err
	}
	s.mu.Lock()
	device, err := s.store.Delete(ctx, deviceID)
	s.mu.Unlock()
	if err != nil {
		return TrustedDevice{}, err
	}
	now := s.now().UTC()
	s.logger.Info("Trusted device removed", "device_id", deviceID)
	s.publish(Event{Type: EventTrustedDeviceRemoved, TrustedDevice: &device, Timestamp: now})
	return device, nil
}

func (s *Service) findOnlinePeer(deviceID string) (models.Device, bool) {
	for _, device := range s.peers.Devices() {
		if device.ID == deviceID && device.Online {
			return device, true
		}
	}
	return models.Device{}, false
}

func (s *Service) pruneExpiredLocked(now time.Time) {
	for requestID, request := range s.requests {
		if now.Before(request.ExpiresAt) {
			continue
		}
		delete(s.requests, requestID)
		delete(s.pendingByDevice, request.DeviceID)
	}
}

func (s *Service) publish(event Event) {
	if s.publisher != nil {
		s.publisher.Publish(event)
	}
}

func generatePlaceholderKey() (string, error) {
	contents := make([]byte, 32)
	if _, err := rand.Read(contents); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(contents), nil
}

func validateUUID(value string) error {
	if _, err := uuid.Parse(value); err != nil {
		return errors.Join(ErrInvalidIdentifier, err)
	}
	return nil
}

func validateTrustedDevice(device TrustedDevice) error {
	if err := validateUUID(device.DeviceID); err != nil {
		return err
	}
	if strings.TrimSpace(device.DeviceName) == "" || len(device.DeviceName) > 128 ||
		strings.TrimSpace(device.Platform) == "" || len(device.Platform) > 32 ||
		strings.TrimSpace(device.PairingKey) == "" || len(device.PairingKey) > 512 ||
		device.PairedAt.IsZero() || device.LastSeen.IsZero() || device.TrustState != TrustStateTrusted {
		return errors.New("invalid trusted device")
	}
	return nil
}
