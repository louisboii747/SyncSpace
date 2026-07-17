package transfer

import (
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/services"
)

type PeerDirectory interface{ Devices() []models.Device }
type DeviceAuthorizer interface {
	IsTrusted(context.Context, string) (bool, error)
	SharedKey(context.Context, string) ([]byte, error)
	PublicKey(context.Context, string) (ed25519.PublicKey, error)
}

type ServiceConfig struct {
	Store         Store
	Peers         PeerDirectory
	Authorizer    DeviceAuthorizer
	Identity      services.Identity
	DataDirectory string
	Publisher     EventPublisher
	Logger        *slog.Logger
	HTTPClient    *http.Client
	MaxConcurrent int
	ChunkWorkers  int
	ChunkSize     int64
	Now           func() time.Time
}

// Service owns the queue, protocol sessions, receiver, resume decisions,
// integrity checks, and all durable lifecycle transitions.
type Service struct {
	mu            sync.Mutex
	store         Store
	peers         PeerDirectory
	authorizer    DeviceAuthorizer
	identity      services.Identity
	dataDirectory string
	publisher     EventPublisher
	logger        *slog.Logger
	httpClient    *http.Client
	peerScheme    string
	peerClients   map[string]*http.Client
	authMu        sync.Mutex
	authNonces    map[string]time.Time
	maxConcurrent int
	chunkWorkers  int
	chunkSize     int64
	now           func() time.Time
	wake          chan struct{}
	active        map[string]context.CancelFunc
	progressMu    sync.Mutex
	stagingMu     sync.Mutex
	stagingActive map[string]int
	throttleMu    sync.Mutex
	throttleDelay time.Duration
}

func NewService(config ServiceConfig) (*Service, error) {
	if config.Store == nil || config.Peers == nil || config.Authorizer == nil {
		return nil, errors.New("transfer store, peer directory, and device authorizer are required")
	}
	if err := config.Identity.Validate(); err != nil {
		return nil, fmt.Errorf("invalid local identity: %w", err)
	}
	if strings.TrimSpace(config.DataDirectory) == "" {
		return nil, errors.New("transfer data directory is required")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	peerScheme := "https"
	if config.HTTPClient != nil {
		peerScheme = "http"
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 2
	}
	if config.ChunkWorkers <= 0 {
		config.ChunkWorkers = 4
	}
	if config.ChunkSize <= 0 {
		config.ChunkSize = DefaultChunkSize
	}
	if config.ChunkSize > MaximumChunkSize {
		return nil, errors.New("configured chunk size exceeds maximum")
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Service{store: config.Store, peers: config.Peers, authorizer: config.Authorizer, identity: config.Identity, dataDirectory: config.DataDirectory, publisher: config.Publisher, logger: config.Logger, httpClient: config.HTTPClient, peerScheme: peerScheme, peerClients: make(map[string]*http.Client), authNonces: make(map[string]time.Time), maxConcurrent: config.MaxConcurrent, chunkWorkers: config.ChunkWorkers, chunkSize: config.ChunkSize, now: config.Now, wake: make(chan struct{}, 1), active: make(map[string]context.CancelFunc), stagingActive: make(map[string]int)}, nil
}

func DetectCapabilities(dataDirectory string) Capabilities {
	return Capabilities{AvailableStorage: availableStorage(dataDirectory), ProtocolVersion: ProtocolVersion, MaxChunkSize: MaximumChunkSize, CompressionSupport: true, TransferSupport: true}
}
func (s *Service) Capabilities() Capabilities { return DetectCapabilities(s.dataDirectory) }

func (s *Service) Run(ctx context.Context) {
	if err := os.MkdirAll(filepath.Join(s.dataDirectory, "incoming"), 0o700); err != nil {
		s.logger.Error("Transfer storage unavailable", "error", err)
		return
	}
	if err := s.recover(ctx); err != nil {
		s.logger.Error("Transfer recovery failed", "error", err)
	}
	ticker := time.NewTicker(time.Second)
	cleanupTicker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	defer cleanupTicker.Stop()
	s.removeExpiredStages(s.now().UTC())
	for {
		select {
		case <-ctx.Done():
			s.stopAll()
			return
		case <-s.wake:
		case <-ticker.C:
		case now := <-cleanupTicker.C:
			s.removeExpiredStages(now.UTC())
		}
		s.dispatch(ctx)
	}
}

func (s *Service) Queue(ctx context.Context, request QueueRequest) (Transfer, error) {
	if err := validateDeviceID(request.DeviceID); err != nil {
		return Transfer{}, err
	}
	if len(request.Paths) == 0 {
		return Transfer{}, fmt.Errorf("%w: paths are required", ErrInvalidRequest)
	}
	trusted, err := s.authorizer.IsTrusted(ctx, request.DeviceID)
	if err != nil {
		return Transfer{}, err
	}
	if !trusted {
		return Transfer{}, ErrUntrustedDevice
	}
	peer, found := s.peer(request.DeviceID)
	if !found || !peer.Online {
		return Transfer{}, fmt.Errorf("%w: device is offline", ErrInvalidRequest)
	}
	if !peer.TransferCapability || peer.SupportedProtocolVersion != ProtocolVersion || peer.MaximumChunkSize <= 0 {
		return Transfer{}, fmt.Errorf("%w: device does not support this transfer protocol", ErrInvalidRequest)
	}
	for _, path := range request.Paths {
		if _, err := os.Lstat(path); err != nil {
			return Transfer{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
	}
	policy := request.ConflictPolicy
	if policy == "" {
		policy = ConflictPrompt
	}
	if !validPolicy(policy) {
		return Transfer{}, fmt.Errorf("%w: invalid conflict policy", ErrInvalidRequest)
	}
	now := s.now().UTC()
	name := filepath.Base(request.Paths[0])
	if len(request.Paths) > 1 {
		name = fmt.Sprintf("%d items", len(request.Paths))
	}
	chunkSize := s.chunkSize
	if peer.MaximumChunkSize < chunkSize {
		chunkSize = peer.MaximumChunkSize
	}
	t := Transfer{ID: uuid.NewString(), Direction: DirectionOutbound, DeviceID: request.DeviceID, DeviceName: peer.Name, RemoteAddress: s.peerAddress(peer), Filename: name, SourcePaths: append([]string(nil), request.Paths...), Status: StatusQueued, CreatedAt: now, UpdatedAt: now, Priority: now.UnixNano(), Approved: true, ConflictPolicy: policy, ChunkSize: chunkSize, Compression: peer.CompressionSupport, ProtocolVersion: ProtocolVersion}
	if err := s.store.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, err
	}
	s.logAndPublish(EventQueueUpdated, t)
	s.signal()
	return t, nil
}

func (s *Service) ReceiveOffer(ctx context.Context, offer Offer, remoteAddress string, authentication PeerAuthentication) (Transfer, error) {
	if _, err := uuid.Parse(offer.TransferID); err != nil {
		return Transfer{}, fmt.Errorf("%w: invalid transfer ID", ErrInvalidRequest)
	}
	if err := validateDeviceID(offer.DeviceID); err != nil {
		return Transfer{}, err
	}
	trusted, err := s.authorizer.IsTrusted(ctx, offer.DeviceID)
	if err != nil {
		return Transfer{}, err
	}
	if !trusted {
		return Transfer{}, ErrUntrustedDevice
	}
	if authentication.DeviceID != offer.DeviceID || !validProtocolTime(authentication.Timestamp, s.now().UTC()) || authentication.Nonce == "" {
		return Transfer{}, fmt.Errorf("%w: malformed offer authentication", ErrUnauthorized)
	}
	sharedKey, err := s.authorizer.SharedKey(ctx, offer.DeviceID)
	if err != nil {
		return Transfer{}, fmt.Errorf("%w: pairing credential unavailable", ErrUnauthorized)
	}
	encodedOffer, err := json.Marshal(offer)
	if err != nil {
		return Transfer{}, err
	}
	if !validOfferSignature(sharedKey, authentication, http.MethodPost, "/v1/transfers/offers", encodedOffer) {
		return Transfer{}, fmt.Errorf("%w: offer signature mismatch", ErrUnauthorized)
	}
	if !s.useAuthenticationNonce(authentication.Nonce, s.now().UTC()) {
		return Transfer{}, fmt.Errorf("%w: replayed offer", ErrUnauthorized)
	}
	peer, found := s.peer(offer.DeviceID)
	if !found || !peer.Online || !sameIP(peer.LocalIP, remoteAddress) {
		return Transfer{}, fmt.Errorf("%w: offer source does not match the discovered device", ErrUntrustedDevice)
	}
	if strings.TrimSpace(offer.DeviceName) == "" || len(offer.DeviceName) > 128 || strings.TrimSpace(offer.Filename) == "" || len(offer.Filename) > 512 {
		return Transfer{}, fmt.Errorf("%w: invalid offer labels", ErrInvalidRequest)
	}
	if offer.ProtocolVersion != ProtocolVersion || offer.ChunkSize <= 0 || offer.ChunkSize > MaximumChunkSize || len(offer.Files) == 0 || offer.Size < 0 || len(offer.SessionToken) < 32 {
		return Transfer{}, fmt.Errorf("%w: unsupported offer", ErrInvalidRequest)
	}
	var total int64
	seen := map[string]struct{}{}
	for i := range offer.Files {
		file := &offer.Files[i]
		if _, err := uuid.Parse(file.ID); err != nil {
			return Transfer{}, fmt.Errorf("%w: invalid file ID", ErrInvalidRequest)
		}
		if err := validateRelativePath(file.RelativePath); err != nil {
			return Transfer{}, err
		}
		key := strings.ToLower(file.RelativePath)
		if _, ok := seen[key]; ok {
			return Transfer{}, fmt.Errorf("%w: duplicate path", ErrInvalidRequest)
		}
		seen[key] = struct{}{}
		if file.Size < 0 || file.ChunkSize != offer.ChunkSize || file.ChunkCount != chunkCount(file.Size, file.ChunkSize) {
			return Transfer{}, fmt.Errorf("%w: invalid file metadata", ErrInvalidRequest)
		}
		if !file.Directory && !validSHA256(file.Checksum) {
			return Transfer{}, fmt.Errorf("%w: invalid checksum", ErrInvalidRequest)
		}
		if file.Directory && (file.Size != 0 || file.ChunkCount != 0) {
			return Transfer{}, fmt.Errorf("%w: invalid directory entry", ErrInvalidRequest)
		}
		total += file.Size
	}
	if total != offer.Size {
		return Transfer{}, fmt.Errorf("%w: total size mismatch", ErrInvalidRequest)
	}
	if existing, err := s.store.GetTransfer(ctx, offer.TransferID); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Transfer{}, err
	}
	now := s.now().UTC()
	t := Transfer{ID: offer.TransferID, Direction: DirectionInbound, DeviceID: offer.DeviceID, DeviceName: offer.DeviceName, RemoteAddress: remoteAddress, Filename: offer.Filename, Files: offer.Files, Size: offer.Size, Status: StatusQueued, CreatedAt: now, UpdatedAt: now, Priority: now.UnixNano(), ApprovalRequired: true, ConflictPolicy: ConflictPrompt, ChunkSize: offer.ChunkSize, Compression: offer.Compression, ProtocolVersion: offer.ProtocolVersion, SessionTokenHash: hashToken(offer.SessionToken)}
	if err := s.store.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, err
	}
	s.logger.Info("Incoming transfer awaiting approval", "transfer_id", t.ID, "device_id", t.DeviceID, "size", t.Size)
	s.logAndPublish(EventQueueUpdated, t)
	return t, nil
}

func (s *Service) List(ctx context.Context) ([]Transfer, error) { return s.store.ListTransfers(ctx) }
func (s *Service) Get(ctx context.Context, id string) (Transfer, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Transfer{}, ErrInvalidRequest
	}
	return s.store.GetTransfer(ctx, id)
}
func (s *Service) DeleteHistory(ctx context.Context) error { return s.store.DeleteHistory(ctx) }

func (s *Service) Accept(ctx context.Context, id string, request AcceptRequest) (Transfer, error) {
	t, err := s.Get(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	if t.Direction != DirectionInbound || t.Status != StatusQueued || !t.ApprovalRequired {
		return Transfer{}, ErrInvalidState
	}
	if strings.TrimSpace(request.DestinationPath) == "" {
		return Transfer{}, fmt.Errorf("%w: destination path is required", ErrInvalidRequest)
	}
	root, err := filepath.Abs(request.DestinationPath)
	if err != nil {
		return Transfer{}, err
	}
	if err = os.MkdirAll(root, 0o700); err != nil {
		return Transfer{}, err
	}
	policy := request.ConflictPolicy
	if policy == "" {
		policy = ConflictPrompt
	}
	if !validPolicy(policy) {
		return Transfer{}, ErrInvalidRequest
	}
	for i := range t.Files {
		target, err := secureJoin(root, t.Files[i].RelativePath)
		if err != nil {
			return Transfer{}, err
		}
		t.Files[i].DestinationPath = target
		if policy == ConflictPrompt {
			if _, statErr := os.Lstat(target); statErr == nil {
				return Transfer{}, fmt.Errorf("%w: %s", ErrConflictResolution, t.Files[i].RelativePath)
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return Transfer{}, statErr
			}
		}
	}
	now := s.now().UTC()
	t.Path = root
	t.Approved = true
	t.ConflictPolicy = policy
	t.Status = StatusReceiving
	t.UpdatedAt = now
	t.StartedAt = &now
	if err = s.store.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, err
	}
	s.logger.Info("Incoming transfer accepted", "transfer_id", t.ID, "device_id", t.DeviceID)
	s.logAndPublish(EventResumed, t)
	return t, nil
}

func (s *Service) Reject(ctx context.Context, id string) (Transfer, error) {
	t, err := s.Get(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	if t.Direction != DirectionInbound || t.Approved || t.Status != StatusQueued {
		return Transfer{}, ErrInvalidState
	}
	return s.finish(ctx, t, StatusCancelled, "", EventQueueUpdated)
}

func (s *Service) Pause(ctx context.Context, id string) (Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	t, err := s.store.GetTransfer(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	switch t.Status {
	case StatusSending, StatusReceiving, StatusConnecting, StatusNegotiating, StatusPreparing, StatusResuming:
	default:
		return Transfer{}, ErrInvalidState
	}
	if cancel := s.active[id]; cancel != nil {
		cancel()
	}
	t.Status = StatusPaused
	t.Speed = 0
	t.ETASeconds = 0
	t.UpdatedAt = s.now().UTC()
	if err = s.store.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, err
	}
	s.logAndPublish(EventPaused, t)
	return t, nil
}

func (s *Service) Resume(ctx context.Context, id string) (Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	t, err := s.store.GetTransfer(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	if t.Status != StatusPaused {
		return Transfer{}, ErrInvalidState
	}
	t.Status = StatusResuming
	t.Error = ""
	t.UpdatedAt = s.now().UTC()
	if t.Direction == DirectionInbound {
		t.Status = StatusReceiving
	}
	if err = s.store.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, err
	}
	s.logAndPublish(EventResumed, t)
	s.signal()
	return t, nil
}

func (s *Service) Cancel(ctx context.Context, id string) (Transfer, error) {
	s.mu.Lock()
	if cancel := s.active[id]; cancel != nil {
		cancel()
	}
	s.mu.Unlock()
	t, err := s.Get(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	switch t.Status {
	case StatusCompleted, StatusCancelled:
		return Transfer{}, ErrInvalidState
	}
	result, finishErr := s.finish(ctx, t, StatusCancelled, "", EventQueueUpdated)
	if finishErr == nil && t.Direction == DirectionOutbound && t.SessionToken != "" && t.RemoteAddress != "" {
		cancelContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.jsonRequest(cancelContext, http.MethodDelete, t.RemoteAddress+"/v1/transfers/"+t.ID, t.DeviceID, t.SessionToken, nil, nil)
		cancel()
	}
	return result, finishErr
}

func (s *Service) Retry(ctx context.Context, id string) (Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	t, err := s.store.GetTransfer(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	if t.Status != StatusFailed {
		return Transfer{}, ErrInvalidState
	}
	t.Error = ""
	t.FinishedAt = nil
	t.Speed = 0
	t.ETASeconds = 0
	t.Status = StatusQueued
	if t.Direction == DirectionInbound && t.Approved {
		t.Status = StatusReceiving
	}
	t.UpdatedAt = s.now().UTC()
	if err = s.store.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, err
	}
	s.logAndPublish(EventQueueUpdated, t)
	s.signal()
	return t, nil
}

func (s *Service) ProtocolStatus(ctx context.Context, id, token string) (ResumeMap, error) {
	t, err := s.Get(ctx, id)
	if err != nil {
		return ResumeMap{}, err
	}
	if !tokenMatches(t.SessionTokenHash, token) {
		return ResumeMap{}, ErrUnauthorized
	}
	chunks, err := s.store.ListChunks(ctx, id)
	if err != nil {
		return ResumeMap{}, err
	}
	result := ResumeMap{TransferID: id, Status: t.Status, Approved: t.Approved, Chunks: make(map[string][]int64)}
	for _, chunk := range chunks {
		if chunk.Status == ChunkComplete {
			result.Chunks[chunk.FileID] = append(result.Chunks[chunk.FileID], chunk.Index)
		}
	}
	return result, nil
}

func (s *Service) ProtocolCancel(ctx context.Context, id, token string) (Transfer, error) {
	t, err := s.Get(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	if !tokenMatches(t.SessionTokenHash, token) {
		return Transfer{}, ErrUnauthorized
	}
	if t.Status == StatusCompleted || t.Status == StatusCancelled {
		return Transfer{}, ErrInvalidState
	}
	return s.finish(ctx, t, StatusCancelled, "cancelled by sender", EventQueueUpdated)
}

func (s *Service) ReceiveChunk(ctx context.Context, id, fileID, token string, index int64, expectedChecksum, contentEncoding string, contents io.Reader) (Transfer, error) {
	t, err := s.Get(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	if !tokenMatches(t.SessionTokenHash, token) {
		return Transfer{}, ErrUnauthorized
	}
	if !t.Approved {
		return Transfer{}, ErrApprovalRequired
	}
	if t.Status != StatusReceiving && t.Status != StatusResuming {
		return Transfer{}, ErrInvalidState
	}
	file, found := findFile(t.Files, fileID)
	if !found || file.Directory || index < 0 || index >= file.ChunkCount || !validSHA256(expectedChecksum) {
		return Transfer{}, ErrInvalidRequest
	}
	expectedSize := file.ChunkSize
	if remaining := file.Size - index*file.ChunkSize; remaining < expectedSize {
		expectedSize = remaining
	}
	var decoded io.Reader = contents
	if contentEncoding != "" {
		if contentEncoding != "gzip" {
			return Transfer{}, fmt.Errorf("%w: unsupported content encoding", ErrInvalidRequest)
		}
		reader, gzipErr := gzip.NewReader(io.LimitReader(contents, MaximumChunkSize+1))
		if gzipErr != nil {
			return Transfer{}, fmt.Errorf("%w: invalid compressed chunk", ErrInvalidRequest)
		}
		defer reader.Close()
		decoded = reader
	}
	limited := io.LimitReader(decoded, expectedSize+1)
	buffer, err := io.ReadAll(limited)
	if err != nil {
		return Transfer{}, err
	}
	if int64(len(buffer)) != expectedSize {
		return Transfer{}, fmt.Errorf("%w: unexpected chunk size", ErrInvalidRequest)
	}
	actual := checksumBytes(buffer)
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expectedChecksum)) != 1 {
		return Transfer{}, ErrChecksumMismatch
	}
	partPath := s.partialPath(t.ID, file.RelativePath)
	if err = os.MkdirAll(filepath.Dir(partPath), 0o700); err != nil {
		return Transfer{}, err
	}
	handle, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Transfer{}, err
	}
	_, writeErr := handle.WriteAt(buffer, index*file.ChunkSize)
	syncErr := handle.Sync()
	closeErr := handle.Close()
	if writeErr != nil {
		return Transfer{}, writeErr
	}
	if syncErr != nil {
		return Transfer{}, syncErr
	}
	if closeErr != nil {
		return Transfer{}, closeErr
	}
	now := s.now().UTC()
	chunk := Chunk{TransferID: id, FileID: fileID, Index: index, Offset: index * file.ChunkSize, Size: int64(len(buffer)), Checksum: actual, Status: ChunkComplete, Attempts: 1, UpdatedAt: now}
	if err = s.store.SaveChunk(ctx, chunk); err != nil {
		return Transfer{}, err
	}
	return s.refreshProgress(ctx, t, now)
}

func (s *Service) Finalize(ctx context.Context, id, token string) (Transfer, error) {
	t, err := s.Get(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	if !tokenMatches(t.SessionTokenHash, token) {
		return Transfer{}, ErrUnauthorized
	}
	if !t.Approved {
		return Transfer{}, ErrApprovalRequired
	}
	if t.Status != StatusReceiving && t.Status != StatusResuming {
		return Transfer{}, ErrInvalidState
	}
	t.Status = StatusVerifying
	t.UpdatedAt = s.now().UTC()
	if err = s.store.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, err
	}
	s.logAndPublish(EventVerification, t)
	chunks, err := s.store.ListChunks(ctx, id)
	if err != nil {
		return Transfer{}, err
	}
	complete := make(map[string]map[int64]bool)
	for _, c := range chunks {
		if c.Status == ChunkComplete {
			if complete[c.FileID] == nil {
				complete[c.FileID] = map[int64]bool{}
			}
			complete[c.FileID][c.Index] = true
		}
	}
	for _, file := range t.Files {
		if file.Directory {
			continue
		}
		if int64(len(complete[file.ID])) != file.ChunkCount {
			return s.fail(ctx, t, fmt.Errorf("missing chunks for %s", file.RelativePath))
		}
		part := s.partialPath(t.ID, file.RelativePath)
		if file.Size == 0 {
			if err := os.MkdirAll(filepath.Dir(part), 0o700); err != nil {
				return s.fail(ctx, t, err)
			}
			handle, err := os.OpenFile(part, os.O_CREATE, 0o600)
			if err != nil {
				return s.fail(ctx, t, err)
			}
			_ = handle.Close()
		}
		checksum, err := checksumFile(part)
		if err != nil {
			return s.fail(ctx, t, err)
		}
		if subtle.ConstantTimeCompare([]byte(checksum), []byte(file.Checksum)) != 1 {
			return s.fail(ctx, t, fmt.Errorf("%w: %s", ErrChecksumMismatch, file.RelativePath))
		}
	}
	for _, file := range t.Files {
		target := file.DestinationPath
		if target == "" {
			return s.fail(ctx, t, errors.New("destination path is missing"))
		}
		if file.Directory {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return s.fail(ctx, t, err)
			}
			continue
		}
		target, err = resolveConflict(target, t.ConflictPolicy)
		if err != nil {
			return s.fail(ctx, t, err)
		}
		if err := commitFile(s.partialPath(t.ID, file.RelativePath), target, t.ConflictPolicy == ConflictOverwrite); err != nil {
			return s.fail(ctx, t, err)
		}
	}
	t.Progress = t.Size
	return s.finish(ctx, t, StatusCompleted, "", EventCompleted)
}

func (s *Service) partialPath(transferID, relative string) string {
	safe, _ := secureJoin(filepath.Join(s.dataDirectory, "incoming", transferID), relative)
	return safe + ".part"
}
func (s *Service) refreshProgress(ctx context.Context, t Transfer, now time.Time) (Transfer, error) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	current, err := s.store.GetTransfer(ctx, t.ID)
	if err != nil {
		return Transfer{}, err
	}
	t = current
	chunks, err := s.store.ListChunks(ctx, t.ID)
	if err != nil {
		return Transfer{}, err
	}
	var progress int64
	for _, c := range chunks {
		if c.Status == ChunkComplete {
			progress += c.Size
		}
	}
	elapsed := time.Second
	if t.StartedAt != nil && now.Sub(*t.StartedAt) > elapsed {
		elapsed = now.Sub(*t.StartedAt)
	}
	t.Progress = progress
	if t.Status == StatusPaused || t.Status == StatusCancelled || t.Status == StatusCompleted || t.Status == StatusFailed {
		t.Speed = 0
		t.ETASeconds = 0
	} else {
		t.Speed = int64(float64(progress) / elapsed.Seconds())
	}
	if t.Speed > 0 && t.Size > progress {
		t.ETASeconds = (t.Size - progress) / t.Speed
	}
	t.UpdatedAt = now
	if err = s.store.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, err
	}
	s.logAndPublish(EventProgress, t)
	return t, nil
}

func (s *Service) finish(ctx context.Context, t Transfer, status Status, message string, event EventType) (Transfer, error) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	now := s.now().UTC()
	t.Status = status
	t.Error = message
	t.Speed = 0
	t.ETASeconds = 0
	t.UpdatedAt = now
	t.FinishedAt = &now
	if err := s.store.SaveTransfer(ctx, t); err != nil {
		return Transfer{}, err
	}
	s.logAndPublish(event, t)
	return t, nil
}
func (s *Service) fail(ctx context.Context, t Transfer, err error) (Transfer, error) {
	result, saveErr := s.finish(ctx, t, StatusFailed, err.Error(), EventFailed)
	if saveErr != nil {
		return Transfer{}, saveErr
	}
	return result, err
}

func (s *Service) recover(ctx context.Context) error {
	items, err := s.store.ListTransfers(ctx)
	if err != nil {
		return err
	}
	for _, t := range items {
		if t.Direction == DirectionOutbound {
			switch t.Status {
			case StatusPreparing, StatusConnecting, StatusNegotiating, StatusSending, StatusResuming:
				t.Status = StatusQueued
				t.UpdatedAt = s.now().UTC()
				t.Error = "recovered after restart"
				if err := s.store.SaveTransfer(ctx, t); err != nil {
					return err
				}
			}
		} else if t.Status == StatusVerifying {
			t.Status = StatusReceiving
			t.UpdatedAt = s.now().UTC()
			if err := s.store.SaveTransfer(ctx, t); err != nil {
				return err
			}
		}
	}
	s.signal()
	return nil
}
func (s *Service) dispatch(parent context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.active) >= s.maxConcurrent {
		return
	}
	items, err := s.store.ListTransfers(parent)
	if err != nil {
		s.logger.Error("Transfer queue load failed", "error", err)
		return
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Priority < items[j].Priority })
	for _, t := range items {
		if len(s.active) >= s.maxConcurrent {
			return
		}
		if t.Direction != DirectionOutbound || (t.Status != StatusQueued && t.Status != StatusResuming) {
			continue
		}
		workerCtx, cancel := context.WithCancel(parent)
		s.active[t.ID] = cancel
		go func(id string) {
			err := s.processOutbound(workerCtx, id)
			s.mu.Lock()
			delete(s.active, id)
			s.mu.Unlock()
			if err != nil && !errors.Is(err, context.Canceled) {
				current, getErr := s.store.GetTransfer(context.Background(), id)
				if getErr == nil && current.Status != StatusPaused && current.Status != StatusCancelled {
					_, _ = s.fail(context.Background(), current, err)
				}
			}
			s.signal()
		}(t.ID)
	}
}
func (s *Service) stopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cancel := range s.active {
		cancel()
	}
}
func (s *Service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *Service) peer(id string) (models.Device, bool) {
	for _, peer := range s.peers.Devices() {
		if peer.ID == id {
			return peer, true
		}
	}
	return models.Device{}, false
}
func (s *Service) logAndPublish(kind EventType, t Transfer) {
	s.logger.Info("Transfer lifecycle", "event", kind, "transfer_id", t.ID, "status", t.Status, "progress", t.Progress, "size", t.Size, "speed", t.Speed)
	if s.publisher != nil {
		s.publisher.Publish(Event{Type: kind, Transfer: t, Timestamp: s.now().UTC()})
	}
}

func (s *Service) waitForThrottle(ctx context.Context) error {
	s.throttleMu.Lock()
	delay := s.throttleDelay
	s.throttleMu.Unlock()
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Service) recordCongestion(failed bool) {
	s.throttleMu.Lock()
	defer s.throttleMu.Unlock()
	if failed {
		if s.throttleDelay == 0 {
			s.throttleDelay = 10 * time.Millisecond
		} else {
			s.throttleDelay *= 2
			if s.throttleDelay > time.Second {
				s.throttleDelay = time.Second
			}
		}
		return
	}
	s.throttleDelay = time.Duration(float64(s.throttleDelay) * 0.8)
	if s.throttleDelay < time.Millisecond {
		s.throttleDelay = 0
	}
}
func findFile(files []File, id string) (File, bool) {
	for _, file := range files {
		if file.ID == id {
			return file, true
		}
	}
	return File{}, false
}
func validPolicy(p ConflictPolicy) bool {
	return p == ConflictPrompt || p == ConflictOverwrite || p == ConflictRename
}
func validSHA256(v string) bool {
	if len(v) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}
func validateDeviceID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("%w: invalid device ID", ErrInvalidRequest)
	}
	return nil
}
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func tokenMatches(hash, token string) bool {
	actual := hashToken(token)
	return len(hash) == len(actual) && subtle.ConstantTimeCompare([]byte(hash), []byte(actual)) == 1
}
func generateToken() (string, error) {
	contents := make([]byte, 32)
	if _, err := rand.Read(contents); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(contents), nil
}
func (s *Service) peerAddress(peer models.Device) string {
	return s.peerScheme + "://" + net.JoinHostPort(strings.TrimSpace(peer.LocalIP), fmt.Sprintf("%d", peer.Port))
}

func (s *Service) peerClient(ctx context.Context, deviceID string) (*http.Client, error) {
	if s.httpClient != nil {
		return s.httpClient, nil
	}
	s.mu.Lock()
	if client := s.peerClients[deviceID]; client != nil {
		s.mu.Unlock()
		return client, nil
	}
	s.mu.Unlock()
	pinnedKey, err := s.authorizer.PublicKey(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("transfer protocol redirects are not allowed")
	}, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, InsecureSkipVerify: true, VerifyConnection: func(state tls.ConnectionState) error {
		if len(state.PeerCertificates) != 1 {
			return errors.New("peer did not present exactly one TLS certificate")
		}
		presented, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !ok || !presented.Equal(pinnedKey) {
			return errors.New("peer TLS identity does not match the paired device")
		}
		return nil
	}}}}
	s.mu.Lock()
	s.peerClients[deviceID] = client
	s.mu.Unlock()
	return client, nil
}

func validProtocolTime(value string, now time.Time) bool {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return false
	}
	difference := now.Unix() - seconds
	if difference < 0 {
		difference = -difference
	}
	return difference <= int64((2 * time.Minute).Seconds())
}

func (s *Service) useAuthenticationNonce(nonce string, now time.Time) bool {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	for value, used := range s.authNonces {
		if now.Sub(used) > 5*time.Minute {
			delete(s.authNonces, value)
		}
	}
	if _, exists := s.authNonces[nonce]; exists {
		return false
	}
	s.authNonces[nonce] = now
	return true
}

func sameIP(left, right string) bool {
	a, b := net.ParseIP(strings.TrimSpace(left)), net.ParseIP(strings.TrimSpace(right))
	return a != nil && b != nil && a.Equal(b)
}

func storageProbePath(path string) string {
	current, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	for {
		if _, err := os.Stat(current); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		current = parent
	}
}

func commitFile(source, target string, overwrite bool) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if overwrite {
		if info, err := os.Lstat(target); err == nil {
			if info.IsDir() {
				return errors.New("cannot overwrite a directory with a file")
			}
			if err := os.Remove(target); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if overwrite {
		output, err = os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	}
	if err != nil {
		return err
	}
	_, copyErr := io.CopyBuffer(output, input, make([]byte, 1<<20))
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Remove(source)
}
