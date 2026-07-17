package database

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateIsOrderedAndIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	version, err := Version(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("schema version=%d want=%d", version, CurrentSchemaVersion)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('trusted_devices') WHERE name IN ('public_key','fingerprint','identity_key_changed')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("cryptographic trust columns=%d", count)
	}
}

func TestMigrateUpgradesLegacyTrustedDeviceTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE trusted_devices (device_id TEXT PRIMARY KEY,device_name TEXT NOT NULL,platform TEXT NOT NULL,pairing_key TEXT NOT NULL,paired_at INTEGER NOT NULL,last_seen INTEGER NOT NULL,trust_state TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('trusted_devices') WHERE name='public_key'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("legacy table was not upgraded")
	}
}
