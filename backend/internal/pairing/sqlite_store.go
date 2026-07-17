package pairing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdatabase "github.com/louisboii747/syncspace/backend/internal/database"
	_ "modernc.org/sqlite"
)

type SQLiteTrustedDeviceStore struct{ database *sql.DB }

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
	return appdatabase.Migrate(ctx, s.database)
}

const trustedSelect = `SELECT device_id,device_name,local_name,platform,public_key,fingerprint,pairing_key,paired_at,last_seen,last_authenticated,trust_state,blocked,identity_key_changed,notes FROM trusted_devices`

func (s *SQLiteTrustedDeviceStore) List(ctx context.Context) ([]TrustedDevice, error) {
	rows, err := s.database.QueryContext(ctx, trustedSelect+` ORDER BY paired_at ASC, device_id ASC`)
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
	return devices, rows.Err()
}

func (s *SQLiteTrustedDeviceStore) Get(ctx context.Context, deviceID string) (TrustedDevice, error) {
	device, err := scanTrustedDevice(s.database.QueryRowContext(ctx, trustedSelect+` WHERE device_id=?`, deviceID))
	if errors.Is(err, sql.ErrNoRows) {
		return TrustedDevice{}, ErrTrustedDeviceNotFound
	}
	return device, err
}

func (s *SQLiteTrustedDeviceStore) Upsert(ctx context.Context, device TrustedDevice) error {
	_, err := s.database.ExecContext(ctx, `INSERT INTO trusted_devices
		(device_id,device_name,local_name,platform,public_key,fingerprint,pairing_key,paired_at,last_seen,last_authenticated,trust_state,blocked,identity_key_changed,notes)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET
		device_name=excluded.device_name,local_name=excluded.local_name,platform=excluded.platform,public_key=excluded.public_key,
		fingerprint=excluded.fingerprint,pairing_key=excluded.pairing_key,paired_at=excluded.paired_at,last_seen=excluded.last_seen,
		last_authenticated=excluded.last_authenticated,trust_state=excluded.trust_state,blocked=excluded.blocked,
		identity_key_changed=excluded.identity_key_changed,notes=excluded.notes`,
		device.DeviceID, device.DeviceName, device.LocalName, device.Platform, device.PublicKey, device.Fingerprint, device.PairingKey,
		device.PairedAt.UTC().UnixMilli(), device.LastSeen.UTC().UnixMilli(), device.LastAuthenticated.UTC().UnixMilli(), device.TrustState, device.Blocked, device.IdentityKeyChanged, device.Notes)
	if err != nil {
		return fmt.Errorf("save trusted device: %w", err)
	}
	return nil
}

func (s *SQLiteTrustedDeviceStore) Delete(ctx context.Context, deviceID string) (TrustedDevice, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return TrustedDevice{}, err
	}
	defer tx.Rollback()
	device, err := scanTrustedDevice(tx.QueryRowContext(ctx, trustedSelect+` WHERE device_id=?`, deviceID))
	if errors.Is(err, sql.ErrNoRows) {
		return TrustedDevice{}, ErrTrustedDeviceNotFound
	}
	if err != nil {
		return TrustedDevice{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM trusted_devices WHERE device_id=?`, deviceID); err != nil {
		return TrustedDevice{}, err
	}
	if err := tx.Commit(); err != nil {
		return TrustedDevice{}, err
	}
	return device, nil
}

type rowScanner interface{ Scan(...any) error }

func scanTrustedDevice(row rowScanner) (TrustedDevice, error) {
	var device TrustedDevice
	var pairedAt, lastSeen, lastAuthenticated int64
	if err := row.Scan(&device.DeviceID, &device.DeviceName, &device.LocalName, &device.Platform, &device.PublicKey, &device.Fingerprint, &device.PairingKey, &pairedAt, &lastSeen, &lastAuthenticated, &device.TrustState, &device.Blocked, &device.IdentityKeyChanged, &device.Notes); err != nil {
		return TrustedDevice{}, err
	}
	device.PairedAt = time.UnixMilli(pairedAt).UTC()
	device.LastSeen = time.UnixMilli(lastSeen).UTC()
	if lastAuthenticated > 0 {
		device.LastAuthenticated = time.UnixMilli(lastAuthenticated).UTC()
	}
	return device, nil
}
