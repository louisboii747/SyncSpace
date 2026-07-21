package pairing

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/services"
)

var (
	ErrPeerNotDiscovered = errors.New("device is not currently discovered")
	ErrAlreadyTrusted    = errors.New("device is already trusted")
	ErrRequestNotFound   = errors.New("pairing request not found")
	ErrInvalidIdentifier = errors.New("identifier must be a UUID")
	ErrProtocol          = errors.New("invalid pairing protocol message")
	ErrIdentityChanged   = errors.New("trusted device identity key changed; forget and re-pair only after verifying the device")
	ErrPairingRateLimit  = errors.New("pairing requests are arriving too quickly")
)

const (
	defaultRequestTTL = 5 * time.Minute
	protocolMaxSkew   = 2 * time.Minute
	pairingCooldown   = 2 * time.Second
	maxPending        = 32
)

type PeerDirectory interface{ Devices() []models.Device }

type PeerTransport interface {
	Begin(context.Context, models.Device, BeginRequest) (BeginResponse, error)
	Proof(context.Context, models.Device, Proof) (PeerDecision, error)
}

type ServiceConfig struct {
	Store      TrustedDeviceStore
	Peers      PeerDirectory
	Identity   services.Identity
	Transport  PeerTransport
	Publisher  EventPublisher
	Logger     *slog.Logger
	RequestTTL time.Duration
	Now        func() time.Time
}

type pairingSession struct {
	request         Request
	secret          []byte
	remotePublicKey string
	peer            models.Device
	response        BeginResponse
	lastProofNonces map[string]time.Time
	trusted         *TrustedDevice
}

type Service struct {
	mu              sync.Mutex
	identityMu      sync.RWMutex
	sessions        map[string]*pairingSession
	pendingByDevice map[string]string
	recentBegins    map[string]time.Time
	lastBeginByHost map[string]time.Time
	store           TrustedDeviceStore
	peers           PeerDirectory
	identity        services.Identity
	transport       PeerTransport
	publisher       EventPublisher
	logger          *slog.Logger
	requestTTL      time.Duration
	now             func() time.Time
}

// SetDisplayName updates the friendly name used in future authenticated
// pairing messages. The signing key and stable identity remain unchanged.
func (s *Service) SetDisplayName(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	s.identityMu.Lock()
	s.identity.Name = name
	s.identityMu.Unlock()
}

func (s *Service) identitySnapshot() services.Identity {
	s.identityMu.RLock()
	defer s.identityMu.RUnlock()
	return s.identity
}

func NewService(config ServiceConfig) (*Service, error) {
	if config.Store == nil || config.Peers == nil {
		return nil, errors.New("trusted device store and peer directory are required")
	}
	if err := config.Identity.Validate(); err != nil {
		return nil, fmt.Errorf("valid cryptographic identity is required: %w", err)
	}
	if config.Transport == nil {
		config.Transport = NewHTTPTransport()
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.RequestTTL <= 0 {
		config.RequestTTL = defaultRequestTTL
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Service{
		sessions: make(map[string]*pairingSession), pendingByDevice: make(map[string]string), recentBegins: make(map[string]time.Time), lastBeginByHost: make(map[string]time.Time),
		store: config.Store, peers: config.Peers, identity: config.Identity, transport: config.Transport, publisher: config.Publisher, logger: config.Logger, requestTTL: config.RequestTTL, now: config.Now,
	}, nil
}

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
		changed := false
		if devices[index].PublicKey == "" || devices[index].Fingerprint == "" || decodeCredential(devices[index]) != nil {
			devices[index].IdentityKeyChanged = true
			changed = true
		}
		if !found {
			if changed {
				if saveErr := s.store.Upsert(ctx, devices[index]); saveErr != nil {
					return nil, saveErr
				}
			}
			continue
		}
		if peer.LastSeen.After(devices[index].LastSeen) {
			devices[index].LastSeen = peer.LastSeen.UTC()
			changed = true
		}
		if peer.IdentityHint != "" && devices[index].Fingerprint != "" && shortFingerprint(devices[index].Fingerprint) != peer.IdentityHint {
			devices[index].IdentityKeyChanged = true
			changed = true
		}
		if changed {
			if saveErr := s.store.Upsert(ctx, devices[index]); saveErr != nil {
				return nil, saveErr
			}
		}
	}
	return devices, nil
}

func (s *Service) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(s.now().UTC())
	result := make([]Request, 0, len(s.sessions))
	for _, session := range s.sessions {
		if session.request.State != RequestStatePaired {
			result = append(result, session.request)
		}
	}
	return result
}

func (s *Service) IsTrusted(ctx context.Context, deviceID string) (bool, error) {
	device, err := s.trusted(ctx, deviceID)
	if errors.Is(err, ErrTrustedDeviceNotFound) {
		return false, nil
	}
	return err == nil && !device.Blocked && !device.IdentityKeyChanged && device.TrustState == TrustStateTrusted && decodeCredential(device) == nil, err
}

func (s *Service) SharedKey(ctx context.Context, deviceID string) ([]byte, error) {
	device, err := s.trusted(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if device.Blocked || device.IdentityKeyChanged || device.TrustState != TrustStateTrusted {
		return nil, ErrIdentityChanged
	}
	return decodeSharedKey(device.PairingKey)
}

func (s *Service) PublicKey(ctx context.Context, deviceID string) (ed25519.PublicKey, error) {
	device, err := s.trusted(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if device.Blocked || device.IdentityKeyChanged {
		return nil, ErrIdentityChanged
	}
	decoded, err := base64.RawURLEncoding.DecodeString(device.PublicKey)
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, errors.New("trusted device public key is invalid")
	}
	return ed25519.PublicKey(decoded), nil
}

func (s *Service) trusted(ctx context.Context, deviceID string) (TrustedDevice, error) {
	if err := validateUUID(deviceID); err != nil {
		return TrustedDevice{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.Get(ctx, deviceID)
}

func (s *Service) RequestPairing(ctx context.Context, deviceID string) (Request, error) {
	if err := validateUUID(deviceID); err != nil {
		return Request{}, err
	}
	peer, found := s.findOnlinePeer(deviceID)
	if !found || !peer.PairingAvailable || peer.IdentityHint == "" {
		return Request{}, ErrPeerNotDiscovered
	}

	s.mu.Lock()
	now := s.now().UTC()
	s.pruneLocked(now)
	if existing, err := s.store.Get(ctx, deviceID); err == nil {
		s.mu.Unlock()
		if existing.IdentityKeyChanged || shortFingerprint(existing.Fingerprint) != peer.IdentityHint {
			return Request{}, ErrIdentityChanged
		}
		return Request{}, ErrAlreadyTrusted
	} else if !errors.Is(err, ErrTrustedDeviceNotFound) {
		s.mu.Unlock()
		return Request{}, err
	}
	if requestID, exists := s.pendingByDevice[deviceID]; exists {
		request := s.sessions[requestID].request
		s.mu.Unlock()
		return request, nil
	}
	if len(s.sessions) >= maxPending {
		s.mu.Unlock()
		return Request{}, ErrPairingRateLimit
	}
	s.mu.Unlock()

	ephemeral, ephemeralPublic, err := newEphemeral()
	if err != nil {
		return Request{}, err
	}
	identity := s.identitySnapshot()
	begin := BeginRequest{ProtocolVersion: ProtocolVersion, RequestID: uuid.NewString(), DeviceID: identity.ID, DeviceName: identity.Name, Platform: identity.Platform, PublicKey: identity.PublicKey, EphemeralKey: ephemeralPublic, Timestamp: fmt.Sprint(now.Unix()), Nonce: uuid.NewString()}
	signRequest(&begin, identity.PrivateKey)
	response, err := s.transport.Begin(ctx, peer, begin)
	if err != nil {
		return Request{}, fmt.Errorf("contact pairing peer: %w", err)
	}
	remotePublic, err := s.validateBeginResponse(peer, begin, response, now)
	if err != nil {
		return Request{}, err
	}
	secret, code, err := derivePairing(ephemeral, response.EphemeralKey, begin, response)
	if err != nil {
		return Request{}, err
	}
	fingerprint := services.IdentityFingerprint(remotePublic)
	request := Request{RequestID: begin.RequestID, DeviceID: peer.ID, DeviceName: response.DeviceName, Platform: response.Platform, Direction: DirectionOutgoing, Fingerprint: fingerprint, VerificationCode: code, RequestedAt: now, ExpiresAt: now.Add(s.requestTTL), State: RequestStatePending}
	s.mu.Lock()
	s.sessions[request.RequestID] = &pairingSession{request: request, secret: secret, remotePublicKey: response.PublicKey, peer: peer, response: response, lastProofNonces: make(map[string]time.Time)}
	s.pendingByDevice[peer.ID] = request.RequestID
	s.mu.Unlock()
	s.logger.Info("Authenticated pairing exchange started", "request_id", request.RequestID, "device_id", request.DeviceID, "direction", request.Direction)
	s.publish(Event{Type: EventPairingRequested, Request: &request, Timestamp: now})
	return request, nil
}

func (s *Service) ReceiveBegin(ctx context.Context, begin BeginRequest, remoteHost string) (BeginResponse, error) {
	now := s.now().UTC()
	if begin.ProtocolVersion != ProtocolVersion || validateUUID(begin.RequestID) != nil || validateUUID(begin.DeviceID) != nil || !validProtocolTimestamp(begin.Timestamp, now.Unix(), int64(protocolMaxSkew.Seconds())) {
		return BeginResponse{}, ErrProtocol
	}
	peer, found := s.findOnlinePeer(begin.DeviceID)
	if !found || !sameIP(peer.LocalIP, remoteHost) || peer.IdentityHint == "" {
		return BeginResponse{}, ErrPeerNotDiscovered
	}
	publicKey, err := verifyRequest(begin)
	if err != nil || shortFingerprint(services.IdentityFingerprint(publicKey)) != peer.IdentityHint {
		return BeginResponse{}, errors.Join(ErrProtocol, errors.New("request identity does not match discovery"))
	}

	s.mu.Lock()
	s.pruneLocked(now)
	if existing := s.sessions[begin.RequestID]; existing != nil && existing.response.RequestID != "" {
		response := existing.response
		s.mu.Unlock()
		return response, nil
	}
	if previous, replayed := s.recentBegins[begin.RequestID]; replayed && now.Sub(previous) < s.requestTTL {
		s.mu.Unlock()
		return BeginResponse{}, ErrProtocol
	}
	if last := s.lastBeginByHost[remoteHost]; !last.IsZero() && now.Sub(last) < pairingCooldown {
		s.mu.Unlock()
		return BeginResponse{}, ErrPairingRateLimit
	}
	if len(s.sessions) >= maxPending {
		s.mu.Unlock()
		return BeginResponse{}, ErrPairingRateLimit
	}
	if trusted, getErr := s.store.Get(ctx, begin.DeviceID); getErr == nil {
		if trusted.PublicKey != begin.PublicKey {
			trusted.IdentityKeyChanged = true
			_ = s.store.Upsert(ctx, trusted)
			s.mu.Unlock()
			return BeginResponse{}, ErrIdentityChanged
		}
		s.mu.Unlock()
		return BeginResponse{}, ErrAlreadyTrusted
	} else if !errors.Is(getErr, ErrTrustedDeviceNotFound) {
		s.mu.Unlock()
		return BeginResponse{}, getErr
	}
	s.lastBeginByHost[remoteHost] = now
	s.recentBegins[begin.RequestID] = now
	s.mu.Unlock()

	ephemeral, ephemeralPublic, err := newEphemeral()
	if err != nil {
		return BeginResponse{}, err
	}
	identity := s.identitySnapshot()
	response := BeginResponse{ProtocolVersion: ProtocolVersion, RequestID: begin.RequestID, DeviceID: identity.ID, DeviceName: identity.Name, Platform: identity.Platform, PublicKey: identity.PublicKey, EphemeralKey: ephemeralPublic, Timestamp: fmt.Sprint(now.Unix()), Nonce: uuid.NewString()}
	signResponse(&response, identity.PrivateKey)
	secret, code, err := derivePairing(ephemeral, begin.EphemeralKey, begin, response)
	if err != nil {
		return BeginResponse{}, err
	}
	request := Request{RequestID: begin.RequestID, DeviceID: begin.DeviceID, DeviceName: begin.DeviceName, Platform: begin.Platform, Direction: DirectionIncoming, Fingerprint: services.IdentityFingerprint(publicKey), VerificationCode: code, RequestedAt: now, ExpiresAt: now.Add(s.requestTTL), State: RequestStatePending}
	s.mu.Lock()
	if existingID := s.pendingByDevice[begin.DeviceID]; existingID != "" && existingID != begin.RequestID {
		delete(s.sessions, existingID)
	}
	s.sessions[begin.RequestID] = &pairingSession{request: request, secret: secret, remotePublicKey: begin.PublicKey, peer: peer, response: response, lastProofNonces: make(map[string]time.Time)}
	s.pendingByDevice[begin.DeviceID] = begin.RequestID
	s.mu.Unlock()
	s.publish(Event{Type: EventPairingRequested, Request: &request, Timestamp: now})
	return response, nil
}

func (s *Service) Accept(ctx context.Context, requestID string) (Decision, error) {
	if err := validateUUID(requestID); err != nil {
		return Decision{}, err
	}
	s.mu.Lock()
	s.pruneLocked(s.now().UTC())
	session := s.sessions[requestID]
	if session == nil {
		s.mu.Unlock()
		return Decision{}, ErrRequestNotFound
	}
	if session.request.State == RequestStateRejected {
		s.mu.Unlock()
		return Decision{Request: session.request}, nil
	}
	session.request.LocalConfirmed = true
	session.request.State = RequestStateConfirming
	direction, peer, secret := session.request.Direction, session.peer, append([]byte(nil), session.secret...)
	if direction == DirectionIncoming && session.request.RemoteConfirmed {
		decision, err := s.finalizeLocked(ctx, session)
		s.mu.Unlock()
		return decision, err
	}
	request := session.request
	s.mu.Unlock()
	if direction == DirectionIncoming {
		s.publish(Event{Type: EventPairingRequested, Request: &request, Timestamp: s.now().UTC()})
		return Decision{Request: request}, nil
	}
	peerDecision, err := s.transport.Proof(ctx, peer, newProof(s.identitySnapshot().ID, requestID, "confirm", secret, s.now().UTC()))
	if err != nil {
		return Decision{Request: request}, fmt.Errorf("confirm pairing with peer: %w", err)
	}
	return s.applyPeerDecision(ctx, requestID, peerDecision)
}

func (s *Service) Refresh(ctx context.Context, requestID string) (Decision, error) {
	if err := validateUUID(requestID); err != nil {
		return Decision{}, err
	}
	s.mu.Lock()
	s.pruneLocked(s.now().UTC())
	session := s.sessions[requestID]
	if session == nil {
		s.mu.Unlock()
		return Decision{}, ErrRequestNotFound
	}
	if session.trusted != nil {
		trusted := *session.trusted
		request := session.request
		s.mu.Unlock()
		return Decision{Request: request, TrustedDevice: &trusted}, nil
	}
	if session.request.Direction != DirectionOutgoing || !session.request.LocalConfirmed {
		request := session.request
		s.mu.Unlock()
		return Decision{Request: request}, nil
	}
	peer, secret := session.peer, append([]byte(nil), session.secret...)
	s.mu.Unlock()
	peerDecision, err := s.transport.Proof(ctx, peer, newProof(s.identitySnapshot().ID, requestID, "status", secret, s.now().UTC()))
	if err != nil {
		return Decision{}, err
	}
	return s.applyPeerDecision(ctx, requestID, peerDecision)
}

func (s *Service) ReceiveProof(ctx context.Context, proof Proof) (PeerDecision, error) {
	now := s.now().UTC()
	if proof.ProtocolVersion != ProtocolVersion || validateUUID(proof.RequestID) != nil || validateUUID(proof.DeviceID) != nil || !validProtocolTimestamp(proof.Timestamp, now.Unix(), int64(protocolMaxSkew.Seconds())) {
		return PeerDecision{}, ErrProtocol
	}
	s.mu.Lock()
	s.pruneLocked(now)
	session := s.sessions[proof.RequestID]
	if session == nil || session.request.DeviceID != proof.DeviceID || !verifyProof(session.secret, proof) {
		s.mu.Unlock()
		return PeerDecision{}, ErrProtocol
	}
	if _, used := session.lastProofNonces[proof.Nonce]; used {
		s.mu.Unlock()
		return PeerDecision{}, ErrProtocol
	}
	session.lastProofNonces[proof.Nonce] = now
	switch proof.Action {
	case "confirm":
		session.request.RemoteConfirmed = true
		if session.request.State == RequestStatePending {
			session.request.State = RequestStateConfirming
		}
	case "reject":
		session.request.State = RequestStateRejected
	case "status":
	default:
		s.mu.Unlock()
		return PeerDecision{}, ErrProtocol
	}
	if session.request.LocalConfirmed && session.request.RemoteConfirmed && session.request.State != RequestStateRejected && session.trusted == nil {
		if _, err := s.finalizeLocked(ctx, session); err != nil {
			s.mu.Unlock()
			return PeerDecision{}, err
		}
	}
	decision := PeerDecision{RequestID: proof.RequestID, State: session.request.State, LocalConfirmed: session.request.LocalConfirmed, RemoteConfirmed: session.request.RemoteConfirmed, Timestamp: fmt.Sprint(now.Unix())}
	decision.MAC = decisionMAC(session.secret, decision)
	s.mu.Unlock()
	return decision, nil
}

func (s *Service) Reject(ctx context.Context, requestID string) (Request, error) {
	if err := validateUUID(requestID); err != nil {
		return Request{}, err
	}
	s.mu.Lock()
	s.pruneLocked(s.now().UTC())
	session := s.sessions[requestID]
	if session == nil {
		s.mu.Unlock()
		return Request{}, ErrRequestNotFound
	}
	session.request.State = RequestStateRejected
	request, peer, secret := session.request, session.peer, append([]byte(nil), session.secret...)
	s.mu.Unlock()
	if request.Direction == DirectionOutgoing {
		_, _ = s.transport.Proof(ctx, peer, newProof(s.identitySnapshot().ID, requestID, "reject", secret, s.now().UTC()))
	}
	s.publish(Event{Type: EventPairingRejected, Request: &request, Timestamp: s.now().UTC()})
	return request, nil
}

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
	s.publish(Event{Type: EventTrustedDeviceRemoved, TrustedDevice: &device, Timestamp: s.now().UTC()})
	return device, nil
}

func (s *Service) SetBlocked(ctx context.Context, deviceID string, blocked bool) (TrustedDevice, error) {
	if err := validateUUID(deviceID); err != nil {
		return TrustedDevice{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	device, err := s.store.Get(ctx, deviceID)
	if err != nil {
		return TrustedDevice{}, err
	}
	device.Blocked = blocked
	if blocked {
		device.TrustState = TrustStateBlocked
	} else {
		device.TrustState = TrustStateTrusted
	}
	if err := s.store.Upsert(ctx, device); err != nil {
		return TrustedDevice{}, err
	}
	return device, nil
}

func (s *Service) applyPeerDecision(ctx context.Context, requestID string, peerDecision PeerDecision) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[requestID]
	if session == nil || peerDecision.RequestID != requestID || !verifyDecision(session.secret, peerDecision) {
		return Decision{}, ErrProtocol
	}
	session.request.RemoteConfirmed = peerDecision.LocalConfirmed
	if peerDecision.State == RequestStateRejected {
		session.request.State = RequestStateRejected
		return Decision{Request: session.request}, nil
	}
	if session.request.LocalConfirmed && session.request.RemoteConfirmed {
		return s.finalizeLocked(ctx, session)
	}
	session.request.State = RequestStateConfirming
	return Decision{Request: session.request}, nil
}

func (s *Service) finalizeLocked(ctx context.Context, session *pairingSession) (Decision, error) {
	now := s.now().UTC()
	trusted := TrustedDevice{DeviceID: session.request.DeviceID, DeviceName: session.request.DeviceName, Platform: session.request.Platform, PublicKey: session.remotePublicKey, Fingerprint: session.request.Fingerprint, PairingKey: base64.RawURLEncoding.EncodeToString(session.secret), PairedAt: now, LastSeen: session.peer.LastSeen.UTC(), LastAuthenticated: now, TrustState: TrustStateTrusted}
	if err := validateTrustedDevice(trusted); err != nil {
		return Decision{}, err
	}
	if err := s.store.Upsert(ctx, trusted); err != nil {
		return Decision{}, err
	}
	session.request.LocalConfirmed = true
	session.request.RemoteConfirmed = true
	session.request.State = RequestStatePaired
	session.trusted = &trusted
	delete(s.pendingByDevice, trusted.DeviceID)
	s.publish(Event{Type: EventPairingAccepted, TrustedDevice: &trusted, Timestamp: now})
	return Decision{Request: session.request, TrustedDevice: &trusted}, nil
}

func (s *Service) validateBeginResponse(peer models.Device, request BeginRequest, response BeginResponse, now time.Time) (ed25519.PublicKey, error) {
	if response.ProtocolVersion != ProtocolVersion || response.RequestID != request.RequestID || response.DeviceID != peer.ID || !validProtocolTimestamp(response.Timestamp, now.Unix(), int64(protocolMaxSkew.Seconds())) {
		return nil, ErrProtocol
	}
	publicKey, err := verifyResponse(response)
	if err != nil || shortFingerprint(services.IdentityFingerprint(publicKey)) != peer.IdentityHint {
		return nil, errors.Join(ErrProtocol, errors.New("response identity does not match discovery"))
	}
	return publicKey, nil
}

func (s *Service) findOnlinePeer(deviceID string) (models.Device, bool) {
	for _, device := range s.peers.Devices() {
		if device.ID == deviceID && device.Online {
			return device, true
		}
	}
	return models.Device{}, false
}
func (s *Service) pruneLocked(now time.Time) {
	for id, session := range s.sessions {
		if now.After(session.request.ExpiresAt) {
			delete(s.pendingByDevice, session.request.DeviceID)
			delete(s.sessions, id)
		}
	}
	for id, seen := range s.recentBegins {
		if now.Sub(seen) > s.requestTTL {
			delete(s.recentBegins, id)
		}
	}
}
func (s *Service) publish(event Event) {
	if s.publisher != nil {
		s.publisher.Publish(event)
	}
}

func newProof(deviceID, requestID, action string, secret []byte, now time.Time) Proof {
	proof := Proof{ProtocolVersion: ProtocolVersion, RequestID: requestID, DeviceID: deviceID, Action: action, Timestamp: fmt.Sprint(now.Unix()), Nonce: uuid.NewString()}
	proof.MAC = proofMAC(secret, proof)
	return proof
}
func validateUUID(value string) error {
	if _, err := uuid.Parse(value); err != nil {
		return errors.Join(ErrInvalidIdentifier, err)
	}
	return nil
}
func sameIP(expected, actual string) bool {
	left, right := net.ParseIP(strings.Trim(expected, "[]")), net.ParseIP(strings.Trim(actual, "[]"))
	return left != nil && right != nil && left.Equal(right)
}

func validateTrustedDevice(device TrustedDevice) error {
	if err := validateUUID(device.DeviceID); err != nil {
		return err
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(device.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || device.Fingerprint != services.IdentityFingerprint(publicKey) {
		return errors.New("invalid trusted public identity")
	}
	if _, err := decodeSharedKey(device.PairingKey); err != nil {
		return err
	}
	if strings.TrimSpace(device.DeviceName) == "" || len(device.DeviceName) > 128 || strings.TrimSpace(device.Platform) == "" || len(device.Platform) > 32 || device.PairedAt.IsZero() || device.LastSeen.IsZero() || (device.TrustState != TrustStateTrusted && device.TrustState != TrustStateBlocked) {
		return errors.New("invalid trusted device")
	}
	return nil
}

func decodeCredential(device TrustedDevice) error {
	publicKey, err := base64.RawURLEncoding.DecodeString(device.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || device.Fingerprint != services.IdentityFingerprint(publicKey) {
		return errors.New("invalid trusted public identity")
	}
	_, err = decodeSharedKey(device.PairingKey)
	return err
}
