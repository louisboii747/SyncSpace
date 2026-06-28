package pairing

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSQLiteTrustedDeviceStorePersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "syncspace.db")
	firstDatabase := openTestDatabase(t, path)
	store, err := NewSQLiteTrustedDeviceStore(ctx, firstDatabase)
	if err != nil {
		t.Fatal(err)
	}
	device := TrustedDevice{
		DeviceID:   uuid.NewString(),
		DeviceName: "Peer-PC",
		Platform:   "windows",
		PairingKey: "placeholder-key",
		PairedAt:   time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
		LastSeen:   time.Date(2026, 6, 28, 12, 1, 0, 0, time.UTC),
		TrustState: TrustStateTrusted,
	}
	if err := store.Upsert(ctx, device); err != nil {
		t.Fatal(err)
	}
	if err := firstDatabase.Close(); err != nil {
		t.Fatal(err)
	}

	secondDatabase := openTestDatabase(t, path)
	defer secondDatabase.Close()
	reopened, err := NewSQLiteTrustedDeviceStore(ctx, secondDatabase)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(ctx, device.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != device {
		t.Fatalf("persisted device = %#v, want %#v", loaded, device)
	}

	deleted, err := reopened.Delete(ctx, device.DeviceID)
	if err != nil || deleted != device {
		t.Fatalf("deleted device = %#v, error = %v", deleted, err)
	}
	if _, err := reopened.Get(ctx, device.DeviceID); !errors.Is(err, ErrTrustedDeviceNotFound) {
		t.Fatalf("expected not found after deletion, got %v", err)
	}
}

func openTestDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	return database
}
