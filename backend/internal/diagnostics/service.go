// Package diagnostics exposes runtime checks and a bounded in-memory log view
// without coupling the application services to the HTTP layer.
package diagnostics

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
	"github.com/louisboii747/syncspace/backend/internal/services"
	"github.com/louisboii747/syncspace/backend/internal/transfer"
)

type Discovery interface {
	Devices() []models.Device
	Self() models.Device
	Refresh()
}
type Trust interface {
	TrustedDevices(context.Context) ([]pairing.TrustedDevice, error)
	IsTrusted(context.Context, string) (bool, error)
}
type Transfers interface {
	List(context.Context) ([]transfer.Transfer, error)
	Queue(context.Context, transfer.QueueRequest) (transfer.Transfer, error)
	DeleteHistory(context.Context) error
}

type Config struct {
	Database       *sql.DB
	DatabasePath   string
	StoragePath    string
	BackendURL     string
	Identity       services.Identity
	Discovery      Discovery
	Trust          Trust
	Transfers      Transfers
	Logs           *LogBuffer
	WebSocketState func() WebSocketState
	DiscoveryState func() bool
	DeveloperMode  bool
}

type WebSocketState struct {
	Discovery int `json:"discovery"`
	Pairing   int `json:"pairing"`
	Transfers int `json:"transfers"`
}
type Check struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}
type Health struct {
	Status    string           `json:"status"`
	Checks    map[string]Check `json:"checks"`
	CheckedAt time.Time        `json:"checkedAt"`
}

type Snapshot struct {
	Health           Health                  `json:"health"`
	LocalDevice      models.Device           `json:"localDevice"`
	Platform         string                  `json:"platform"`
	BackendURL       string                  `json:"backendUrl"`
	WebSockets       WebSocketState          `json:"webSockets"`
	DiscoveryRunning bool                    `json:"discoveryRunning"`
	KnownDevices     []models.Device         `json:"knownDevices"`
	TrustedDevices   []pairing.TrustedDevice `json:"trustedDevices"`
	ActiveTransfers  []transfer.Transfer     `json:"activeTransfers"`
	TransferQueue    []transfer.Transfer     `json:"transferQueue"`
	StoragePath      string                  `json:"storagePath"`
	DatabasePath     string                  `json:"databasePath"`
	ProtocolVersion  int                     `json:"protocolVersion"`
	DeveloperMode    bool                    `json:"developerMode"`
	Logs             []LogEntry              `json:"logs"`
	LastErrors       []LogEntry              `json:"lastErrors"`
}

type Service struct{ config Config }

func New(config Config) (*Service, error) {
	if config.Database == nil || config.Discovery == nil || config.Trust == nil || config.Transfers == nil {
		return nil, fmt.Errorf("database and application services are required")
	}
	return &Service{config: config}, nil
}

func (s *Service) Health(ctx context.Context) Health {
	checks := map[string]Check{}
	check := func(name string, err error, okDetail string) {
		if err != nil {
			checks[name] = Check{OK: false, Detail: err.Error()}
		} else {
			checks[name] = Check{OK: true, Detail: okDetail}
		}
	}
	check("database", s.config.Database.PingContext(ctx), "SQLite is reachable")
	check("storage", writable(s.config.StoragePath), "storage directory is writable")
	ws := WebSocketState{}
	if s.config.WebSocketState != nil {
		ws = s.config.WebSocketState()
		checks["websocket"] = Check{OK: true, Detail: fmt.Sprintf("active clients: discovery=%d pairing=%d transfers=%d", ws.Discovery, ws.Pairing, ws.Transfers)}
	} else {
		checks["websocket"] = Check{OK: false, Detail: "WebSocket brokers are unavailable"}
	}
	discoveryRunning := s.config.DiscoveryState == nil || s.config.DiscoveryState()
	checks["discovery"] = Check{OK: discoveryRunning, Detail: map[bool]string{true: "discovery service is running", false: "discovery service is stopped"}[discoveryRunning]}
	_, err := s.config.Transfers.List(ctx)
	check("transfer", err, "transfer service and queue store are ready")
	check("identity", s.config.Identity.Validate(), "device identity exists")
	_, err = s.config.Trust.TrustedDevices(ctx)
	check("trustedDeviceStore", err, "trusted device store is readable")
	checks["frontendBackend"] = Check{OK: true, Detail: "this request reached the backend"}
	status := "ok"
	for _, item := range checks {
		if !item.OK {
			status = "degraded"
			break
		}
	}
	return Health{Status: status, Checks: checks, CheckedAt: time.Now().UTC()}
}

func (s *Service) Snapshot(ctx context.Context) Snapshot {
	trusted, _ := s.config.Trust.TrustedDevices(ctx)
	transfers, _ := s.config.Transfers.List(ctx)
	active := make([]transfer.Transfer, 0)
	queue := make([]transfer.Transfer, 0)
	for _, item := range transfers {
		if item.Status != transfer.StatusCompleted && item.Status != transfer.StatusCancelled {
			active = append(active, item)
		}
		if item.Status == transfer.StatusQueued || item.Status == transfer.StatusPaused || item.Status == transfer.StatusResuming {
			queue = append(queue, item)
		}
	}
	logs := []LogEntry{}
	errors := []LogEntry{}
	if s.config.Logs != nil {
		logs = s.config.Logs.Entries(80)
		errors = s.config.Logs.Errors(20)
	}
	ws := WebSocketState{}
	if s.config.WebSocketState != nil {
		ws = s.config.WebSocketState()
	}
	running := s.config.DiscoveryState == nil || s.config.DiscoveryState()
	return Snapshot{Health: s.Health(ctx), LocalDevice: s.config.Discovery.Self(), Platform: runtime.GOOS, BackendURL: s.config.BackendURL, WebSockets: ws, DiscoveryRunning: running, KnownDevices: s.config.Discovery.Devices(), TrustedDevices: trusted, ActiveTransfers: active, TransferQueue: queue, StoragePath: s.config.StoragePath, DatabasePath: s.config.DatabasePath, ProtocolVersion: transfer.ProtocolVersion, DeveloperMode: s.config.DeveloperMode, Logs: logs, LastErrors: errors}
}

func (s *Service) Export(ctx context.Context) ([]byte, error) {
	snapshot := s.Snapshot(ctx)
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	entry, err := archive.Create("diagnostics.json")
	if err != nil {
		return nil, err
	}
	encoder := json.NewEncoder(entry)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(snapshot); err != nil {
		return nil, err
	}
	logEntry, err := archive.Create("logs.txt")
	if err != nil {
		return nil, err
	}
	for _, item := range snapshot.Logs {
		_, _ = fmt.Fprintf(logEntry, "%s %-5s %s %s\n", item.Time.Format(time.RFC3339), item.Level, item.Message, formatAttributes(item.Attributes))
	}
	if err = archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (s *Service) QueueTestTransfer(ctx context.Context) (transfer.Transfer, error) {
	return s.QueueTestTransferTo(ctx, "")
}

// QueueTestTransferTo creates a deterministic local seed and queues it to a
// specific peer. An empty device ID selects the first suitable trusted peer.
func (s *Service) QueueTestTransferTo(ctx context.Context, deviceID string) (transfer.Transfer, error) {
	for _, device := range s.config.Discovery.Devices() {
		if deviceID != "" && device.ID != deviceID {
			continue
		}
		trusted, err := s.config.Trust.IsTrusted(ctx, device.ID)
		if err != nil || !trusted || !device.Online || !device.TransferCapability {
			continue
		}
		seedDir := filepath.Join(s.config.StoragePath, "developer-seed")
		if err := os.MkdirAll(seedDir, 0o700); err != nil {
			return transfer.Transfer{}, err
		}
		path := filepath.Join(seedDir, "syncspace-diagnostics-test.txt")
		contents := []byte("SyncSpace diagnostics transfer generated at " + time.Now().UTC().Format(time.RFC3339Nano) + "\n")
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			return transfer.Transfer{}, err
		}
		return s.config.Transfers.Queue(ctx, transfer.QueueRequest{DeviceID: device.ID, Paths: []string{path}, ConflictPolicy: transfer.ConflictRename})
	}
	return transfer.Transfer{}, fmt.Errorf("no online trusted transfer-capable device is available")
}

func (s *Service) ClearTestData(ctx context.Context) error {
	return s.config.Transfers.DeleteHistory(ctx)
}

func writable(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(path, ".syncspace-health-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if _, err = io.WriteString(file, "ok"); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	removeErr := os.Remove(name)
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

type LogEntry struct {
	Time       time.Time         `json:"time"`
	Level      string            `json:"level"`
	Message    string            `json:"message"`
	Attributes map[string]string `json:"attributes,omitempty"`
}
type logState struct {
	mu      sync.Mutex
	entries []LogEntry
	limit   int
}
type LogBuffer struct {
	delegate   slog.Handler
	state      *logState
	attributes []slog.Attr
	group      string
}

func NewLogBuffer(delegate slog.Handler, limit int) *LogBuffer {
	if delegate == nil {
		delegate = slog.NewTextHandler(io.Discard, nil)
	}
	if limit <= 0 {
		limit = 500
	}
	return &LogBuffer{delegate: delegate, state: &logState{limit: limit}}
}
func (h *LogBuffer) Enabled(ctx context.Context, level slog.Level) bool {
	return h.delegate.Enabled(ctx, level)
}
func (h *LogBuffer) Handle(ctx context.Context, record slog.Record) error {
	attrs := make(map[string]string)
	for _, attr := range h.attributes {
		attrs[attr.Key] = fmt.Sprint(attr.Value.Any())
	}
	record.Attrs(func(attr slog.Attr) bool {
		key := attr.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		attrs[key] = fmt.Sprint(attr.Value.Any())
		return true
	})
	entry := LogEntry{Time: record.Time.UTC(), Level: record.Level.String(), Message: record.Message, Attributes: attrs}
	h.state.mu.Lock()
	h.state.entries = append(h.state.entries, entry)
	if len(h.state.entries) > h.state.limit {
		h.state.entries = append([]LogEntry(nil), h.state.entries[len(h.state.entries)-h.state.limit:]...)
	}
	h.state.mu.Unlock()
	return h.delegate.Handle(ctx, record)
}
func (h *LogBuffer) WithAttrs(attrs []slog.Attr) slog.Handler {
	copyAttrs := append(append([]slog.Attr(nil), h.attributes...), attrs...)
	return &LogBuffer{delegate: h.delegate.WithAttrs(attrs), state: h.state, attributes: copyAttrs, group: h.group}
}
func (h *LogBuffer) WithGroup(name string) slog.Handler {
	group := name
	if h.group != "" {
		group = h.group + "." + name
	}
	return &LogBuffer{delegate: h.delegate.WithGroup(name), state: h.state, attributes: h.attributes, group: group}
}
func (h *LogBuffer) Entries(limit int) []LogEntry {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	if limit <= 0 || limit > len(h.state.entries) {
		limit = len(h.state.entries)
	}
	return append([]LogEntry(nil), h.state.entries[len(h.state.entries)-limit:]...)
}
func (h *LogBuffer) Errors(limit int) []LogEntry {
	entries := h.Entries(0)
	result := make([]LogEntry, 0)
	for i := len(entries) - 1; i >= 0 && len(result) < limit; i-- {
		if entries[i].Level == slog.LevelError.String() {
			result = append(result, entries[i])
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Time.Before(result[j].Time) })
	return result
}
func formatAttributes(attrs map[string]string) string {
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%s=%s", key, attrs[key])
	}
	return b.String()
}
