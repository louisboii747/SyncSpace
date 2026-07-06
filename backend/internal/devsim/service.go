// Package devsim provides deterministic local peers and protocol-level fault
// simulation. It is disabled unless SYNCSPACE_DEV_MODE is explicitly enabled.
package devsim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/transfer"
)

type Directory interface {
	Devices() []models.Device
	Self() models.Device
	Refresh()
}

type Scenario string

const (
	ScenarioOnline          Scenario = "online"
	ScenarioOffline         Scenario = "offline"
	ScenarioTrusted         Scenario = "trusted"
	ScenarioUntrusted       Scenario = "untrusted"
	ScenarioSlow            Scenario = "slow"
	ScenarioFlaky           Scenario = "flaky"
	ScenarioRejected        Scenario = "rejected"
	ScenarioInterrupted     Scenario = "interrupted"
	ScenarioDiskFull        Scenario = "disk_full"
	ScenarioChecksumFailure Scenario = "checksum_failure"
)

type Peer struct {
	Device   models.Device `json:"device"`
	Scenario Scenario      `json:"scenario"`
}

type Config struct {
	Enabled     bool
	Base        Directory
	Logger      *slog.Logger
	StaticPeers []models.Device
}

// Service merges real discovery with deterministic local and simulated peers.
type Service struct {
	mu      sync.RWMutex
	enabled bool
	base    Directory
	logger  *slog.Logger
	peers   map[string]Peer
	servers map[string]*httptest.Server
}

func New(config Config) (*Service, error) {
	if config.Base == nil {
		return nil, errors.New("base discovery directory is required")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	s := &Service{enabled: config.Enabled, base: config.Base, logger: config.Logger, peers: make(map[string]Peer), servers: make(map[string]*httptest.Server)}
	for _, peer := range config.StaticPeers {
		if err := validateDevice(peer); err != nil {
			return nil, fmt.Errorf("invalid static peer: %w", err)
		}
		s.peers[peer.ID] = Peer{Device: peer, Scenario: ScenarioOnline}
	}
	return s, nil
}

func (s *Service) Enabled() bool       { return s.enabled }
func (s *Service) Self() models.Device { return s.base.Self() }
func (s *Service) Refresh()            { s.base.Refresh() }

func (s *Service) Devices() []models.Device {
	byID := make(map[string]models.Device)
	for _, device := range s.base.Devices() {
		byID[device.ID] = device
	}
	s.mu.RLock()
	for id, peer := range s.peers {
		device := peer.Device
		device.LastSeen = time.Now().UTC()
		byID[id] = device
	}
	s.mu.RUnlock()
	result := make([]models.Device, 0, len(byID))
	for _, device := range byID {
		result = append(result, device)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result
}

func (s *Service) List() []Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Peer, 0, len(s.peers))
	for _, peer := range s.peers {
		result = append(result, peer)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Device.Name < result[j].Device.Name })
	return result
}

// Simulate creates a real in-process HTTP peer implementing the transfer wire
// protocol and the requested deterministic fault behaviour.
func (s *Service) Simulate(ctx context.Context, name string, scenario Scenario) (Peer, error) {
	if !s.enabled {
		return Peer{}, errors.New("developer mode is disabled")
	}
	if !validScenario(scenario) {
		return Peer{}, fmt.Errorf("unknown simulator scenario %q", scenario)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Simulated " + strings.ReplaceAll(string(scenario), "_", " ")
	}
	id := uuid.NewString()
	receiver := newReceiver(scenario, s.logger)
	server := httptest.NewServer(receiver)
	parsed, _ := url.Parse(server.URL)
	port := parsed.Port()
	var portNumber int
	_, _ = fmt.Sscanf(port, "%d", &portNumber)
	online := scenario != ScenarioOffline
	state := models.ConnectionOnline
	if !online {
		state = models.ConnectionOffline
	}
	device := models.Device{ID: id, Name: name, Type: "simulator", Platform: "simulated", LocalIP: "127.0.0.1", Port: portNumber, AppVersion: "dev-simulator", LastSeen: time.Now().UTC(), Online: online, ConnectionState: state, AvailableStorage: 1 << 40, TransferCapability: online, SupportedProtocolVersion: transfer.ProtocolVersion, MaximumChunkSize: transfer.MaximumChunkSize, CompressionSupport: true}
	peer := Peer{Device: device, Scenario: scenario}
	s.mu.Lock()
	s.peers[id] = peer
	s.servers[id] = server
	s.mu.Unlock()
	s.logger.Info("Simulated device started", "device_id", id, "scenario", scenario, "address", server.URL)
	return peer, nil
}

func (s *Service) Remove(id string) error {
	if !s.enabled {
		return errors.New("developer mode is disabled")
	}
	s.mu.Lock()
	server, found := s.servers[id]
	if _, exists := s.peers[id]; !exists {
		s.mu.Unlock()
		return errors.New("simulated device not found")
	}
	delete(s.peers, id)
	delete(s.servers, id)
	s.mu.Unlock()
	if found {
		server.Close()
	}
	return nil
}

func (s *Service) Close() {
	s.mu.Lock()
	servers := s.servers
	s.servers = make(map[string]*httptest.Server)
	s.peers = make(map[string]Peer)
	s.mu.Unlock()
	for _, server := range servers {
		server.Close()
	}
}

func validScenario(value Scenario) bool {
	switch value {
	case ScenarioOnline, ScenarioOffline, ScenarioTrusted, ScenarioUntrusted, ScenarioSlow, ScenarioFlaky, ScenarioRejected, ScenarioInterrupted, ScenarioDiskFull, ScenarioChecksumFailure:
		return true
	default:
		return false
	}
}

func validateDevice(device models.Device) error {
	if _, err := uuid.Parse(device.ID); err != nil {
		return err
	}
	if device.Name == "" || device.LocalIP == "" || device.Port < 1 {
		return errors.New("device identity, address, and port are required")
	}
	return nil
}

type receiver struct {
	mu       sync.Mutex
	scenario Scenario
	logger   *slog.Logger
	sessions map[string]transfer.Offer
	attempts map[string]int
}

func newReceiver(scenario Scenario, logger *slog.Logger) *receiver {
	return &receiver{scenario: scenario, logger: logger, sessions: make(map[string]transfer.Offer), attempts: make(map[string]int)}
}

func (r *receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r.scenario == ScenarioSlow {
		time.Sleep(350 * time.Millisecond)
	}
	parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if req.Method == http.MethodPost && req.URL.Path == "/v1/transfers/offers" {
		r.offer(w, req)
		return
	}
	if len(parts) >= 3 && parts[0] == "v1" && parts[1] == "transfers" {
		id := parts[2]
		if len(parts) == 4 && parts[3] == "status" && req.Method == http.MethodGet {
			r.status(w, id)
			return
		}
		if len(parts) == 4 && parts[3] == "complete" && req.Method == http.MethodPost {
			r.complete(w, id)
			return
		}
		if len(parts) == 7 && parts[3] == "files" && parts[5] == "chunks" && req.Method == http.MethodPut {
			r.chunk(w, id, parts[4]+"/"+parts[6], req)
			return
		}
		if len(parts) == 3 && req.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	http.NotFound(w, req)
}

func (r *receiver) offer(w http.ResponseWriter, req *http.Request) {
	if r.scenario == ScenarioRejected {
		http.Error(w, "simulated receiver rejected transfer", http.StatusConflict)
		return
	}
	var offer transfer.Offer
	if err := json.NewDecoder(io.LimitReader(req.Body, 16<<20)).Decode(&offer); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	r.sessions[offer.TransferID] = offer
	r.mu.Unlock()
	writeJSON(w, http.StatusCreated, transfer.OfferResponse{TransferID: offer.TransferID, Status: transfer.StatusReceiving, Approved: true})
}

func (r *receiver) status(w http.ResponseWriter, id string) {
	r.mu.Lock()
	_, ok := r.sessions[id]
	r.mu.Unlock()
	if !ok {
		http.NotFound(w, nil)
		return
	}
	writeJSON(w, http.StatusOK, transfer.ResumeMap{TransferID: id, Status: transfer.StatusReceiving, Approved: true, Chunks: map[string][]int64{}})
}

func (r *receiver) chunk(w http.ResponseWriter, id, key string, req *http.Request) {
	r.mu.Lock()
	_, ok := r.sessions[id]
	attemptKey := id + "/" + key
	r.attempts[attemptKey]++
	attempt := r.attempts[attemptKey]
	r.mu.Unlock()
	if !ok {
		http.NotFound(w, req)
		return
	}
	switch r.scenario {
	case ScenarioInterrupted:
		http.Error(w, "simulated connection interruption", http.StatusServiceUnavailable)
		return
	case ScenarioDiskFull:
		http.Error(w, "simulated disk full", http.StatusInsufficientStorage)
		return
	case ScenarioFlaky:
		if attempt == 1 {
			http.Error(w, "simulated transient failure", http.StatusServiceUnavailable)
			return
		}
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(req.Body, transfer.MaximumChunkSize+1))
	w.WriteHeader(http.StatusNoContent)
}

func (r *receiver) complete(w http.ResponseWriter, id string) {
	r.mu.Lock()
	offer, ok := r.sessions[id]
	r.mu.Unlock()
	if !ok {
		http.NotFound(w, nil)
		return
	}
	if r.scenario == ScenarioChecksumFailure {
		http.Error(w, "simulated checksum mismatch", http.StatusUnprocessableEntity)
		return
	}
	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, transfer.Transfer{ID: id, Direction: transfer.DirectionInbound, DeviceID: offer.DeviceID, DeviceName: offer.DeviceName, Filename: offer.Filename, Size: offer.Size, Progress: offer.Size, Status: transfer.StatusCompleted, CreatedAt: now, UpdatedAt: now, FinishedAt: &now, Approved: true, ProtocolVersion: transfer.ProtocolVersion})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
