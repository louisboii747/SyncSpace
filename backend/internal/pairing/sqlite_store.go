package pairing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteTrustedDeviceStore persists trusted devices in the shared local
// SyncSpace SQLite database.
type SQLiteTrustedDeviceStore struct {
	database *sql.DB
}

// NewSQLiteTrustedDeviceStore creates the schema and returns a SQLite-backed
// trust store.
func NewSQLiteTrustedDeviceStore(ctx context.Context, database *sql.DB) (*SQLiteTrustedDeviceStore, error) {
	if database == nil {
		return nil, errors.New("database is required")
	}
	store := &SQLiteTrustedDeviceStore{database: database}
	if err := store.migrate(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLiteTrustedDeviceStore) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS trusted_devices (
			device_id TEXT PRIMARY KEY,
			device_name TEXT NOT NULL,
			platform TEXT NOT NULL,
			pairing_key TEXT NOT NULL,
			paired_at INTEGER NOT NULL,
			last_seen INTEGER NOT NULL,
			trust_state TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize trusted device store: %w", err)
		}
	}
	return nil
}

// List returns all trusted devices in deterministic pairing order.
func (s *SQLiteTrustedDeviceStore) List(ctx context.Context) ([]TrustedDevice, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT device_id, device_name, platform, pairing_key, paired_at, last_seen, trust_state
		FROM trusted_devices
		ORDER BY paired_at ASC, device_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list trusted devices: %w", err)
	}
	defer rows.Close()

	devices := make([]TrustedDevice, 0)
	for rows.Next() {
		device, err := scanTrustedDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trusted devices: %w", err)
	}
	return devices, nil
}

// Get returns one trusted device by permanent device ID.
func (s *SQLiteTrustedDeviceStore) Get(ctx context.Context, deviceID string) (TrustedDevice, error) {
	row := s.database.QueryRowContext(ctx, `
		SELECT device_id, device_name, platform, pairing_key, paired_at, last_seen, trust_state
		FROM trusted_devices WHERE device_id = ?`, deviceID)
	device, err := scanTrustedDevice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TrustedDevice{}, ErrTrustedDeviceNotFound
	}
	return device, err
}

// Upsert atomically creates or refreshes a trusted device record.
func (s *SQLiteTrustedDeviceStore) Upsert(ctx context.Context, device TrustedDevice) error {
	_, err := s.database.ExecContext(ctx, `
		INSERT INTO trusted_devices
			(device_id, device_name, platform, pairing_key, paired_at, last_seen, trust_state)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET
			device_name = excluded.device_name,
			platform = excluded.platform,
			pairing_key = excluded.pairing_key,
			paired_at = excluded.paired_at,
			last_seen = excluded.last_seen,
			trust_state = excluded.trust_state`,
		device.DeviceID,
		device.DeviceName,
		device.Platform,
		device.PairingKey,
		device.PairedAt.UTC().UnixMilli(),
		device.LastSeen.UTC().UnixMilli(),
		device.TrustState,
	)
	if err != nil {
		return fmt.Errorf("save trusted device: %w", err)
	}
	return nil
}

// Delete removes local trust and returns the deleted record.
func (s *SQLiteTrustedDeviceStore) Delete(ctx context.Context, deviceID string) (TrustedDevice, error) {
	transaction, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return TrustedDevice{}, fmt.Errorf("begin trusted device deletion: %w", err)
	}
	defer transaction.Rollback()

	row := transaction.QueryRowContext(ctx, `
		SELECT device_id, device_name, platform, pairing_key, paired_at, last_seen, trust_state
		FROM trusted_devices WHERE device_id = ?`, deviceID)
	device, err := scanTrustedDevice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TrustedDevice{}, ErrTrustedDeviceNotFound
	}
	if err != nil {
		return TrustedDevice{}, err
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM trusted_devices WHERE device_id = ?`, deviceID); err != nil {
		return TrustedDevice{}, fmt.Errorf("delete trusted device: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return TrustedDevice{}, fmt.Errorf("commit trusted device deletion: %w", err)
	}
	return device, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanTrustedDevice(row rowScanner) (TrustedDevice, error) {
	var device TrustedDevice
	var pairedAt, lastSeen int64
	if err := row.Scan(
		&device.DeviceID,
		&device.DeviceName,
		&device.Platform,
		&device.PairingKey,
		&pairedAt,
		&lastSeen,
		&device.TrustState,
	); err != nil {
		return TrustedDevice{}, err
	}
	device.PairedAt = time.UnixMilli(pairedAt).UTC()
	device.LastSeen = time.UnixMilli(lastSeen).UTC()
	return device, nil
}
