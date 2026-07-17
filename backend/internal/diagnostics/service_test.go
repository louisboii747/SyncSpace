package diagnostics

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
	"github.com/louisboii747/syncspace/backend/internal/services"
	"github.com/louisboii747/syncspace/backend/internal/transfer"
	_ "modernc.org/sqlite"
)

type diagnosticDiscovery struct{ self models.Device }

func (d diagnosticDiscovery) Devices() []models.Device { return []models.Device{d.self} }
func (d diagnosticDiscovery) Self() models.Device      { return d.self }
func (diagnosticDiscovery) Refresh()                   {}

type diagnosticTrust struct{}

func (diagnosticTrust) TrustedDevices(context.Context) ([]pairing.TrustedDevice, error) {
	return nil, nil
}
func (diagnosticTrust) IsTrusted(context.Context, string) (bool, error) { return true, nil }

type diagnosticTransfers struct{ items []transfer.Transfer }

func newDiagnosticIdentity(t *testing.T) services.Identity {
	t.Helper()
	identity, err := services.NewFileIdentityStore(filepath.Join(t.TempDir(), "identity.json")).LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	identity.ID, identity.Name, identity.Platform = "00000000-0000-4000-8000-00000000000a", "Device A", "test"
	return identity
}

func (d diagnosticTransfers) List(context.Context) ([]transfer.Transfer, error) {
	return d.items, nil
}
func (diagnosticTransfers) Queue(context.Context, transfer.QueueRequest) (transfer.Transfer, error) {
	return transfer.Transfer{}, nil
}
func (diagnosticTransfers) DeleteHistory(context.Context) error { return nil }

func TestHealthSnapshotAndExportDescribeAWorkingRuntime(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	storage := filepath.Join(t.TempDir(), "transfers")
	identity := newDiagnosticIdentity(t)
	device := models.Device{ID: identity.ID, Name: identity.Name, Type: identity.Type, Platform: identity.Platform, Online: true, TransferCapability: true}
	buffer := NewLogBuffer(slog.NewTextHandler(io.Discard, nil), 20)
	logger := slog.New(buffer)
	logger.Info("Discovery started", "service", "test")
	logger.Error("simulated failure", "transfer_id", "abc")
	service, err := New(Config{
		Database: database, DatabasePath: "memory.db", StoragePath: storage,
		BackendURL: "http://127.0.0.1:8384", Identity: identity,
		Discovery: diagnosticDiscovery{self: device}, Trust: diagnosticTrust{},
		Transfers: diagnosticTransfers{items: []transfer.Transfer{{ID: "queued", Status: transfer.StatusQueued}}},
		Logs:      buffer, DeveloperMode: true, DiscoveryState: func() bool { return true },
		WebSocketState: func() WebSocketState { return WebSocketState{Transfers: 1} },
	})
	if err != nil {
		t.Fatal(err)
	}
	health := service.Health(context.Background())
	if health.Status != "ok" {
		t.Fatalf("health=%#v", health)
	}
	for name, check := range health.Checks {
		if !check.OK {
			t.Fatalf("check %s failed: %#v", name, check)
		}
	}
	snapshot := service.Snapshot(context.Background())
	if !snapshot.DeveloperMode || snapshot.WebSockets.Transfers != 1 || len(snapshot.TransferQueue) != 1 || len(snapshot.LastErrors) != 1 {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	contents, err := service.Export(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range archive.File {
		names[file.Name] = true
	}
	if !names["diagnostics.json"] || !names["logs.txt"] {
		t.Fatalf("archive entries=%v", names)
	}
}

func TestHealthReportsStoppedDiscoveryAndUnwritableStorage(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err = os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity := newDiagnosticIdentity(t)
	service, err := New(Config{
		Database: database, StoragePath: file, Identity: identity,
		Discovery: diagnosticDiscovery{}, Trust: diagnosticTrust{}, Transfers: diagnosticTransfers{},
		DiscoveryState: func() bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	health := service.Health(context.Background())
	if health.Status != "degraded" || health.Checks["storage"].OK || health.Checks["discovery"].OK {
		t.Fatalf("health=%#v", health)
	}
}
