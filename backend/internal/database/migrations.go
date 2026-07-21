// Package database owns the authoritative SQLite schema and ordered
// migrations. Feature stores call this single migrator instead of scattering
// ad-hoc CREATE TABLE calls across packages.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

const CurrentSchemaVersion = 4

type migration struct {
	version    int
	name       string
	statements []string
	apply      func(context.Context, *sql.Tx) error
}

var migrations = []migration{
	{version: 1, name: "trusted devices", statements: []string{`CREATE TABLE IF NOT EXISTS trusted_devices (
		device_id TEXT PRIMARY KEY, device_name TEXT NOT NULL, platform TEXT NOT NULL, pairing_key TEXT NOT NULL,
		paired_at INTEGER NOT NULL, last_seen INTEGER NOT NULL, trust_state TEXT NOT NULL
	)`}},
	{version: 2, name: "durable transfers", statements: []string{
		`CREATE TABLE IF NOT EXISTS transfers (
			id TEXT PRIMARY KEY, direction TEXT NOT NULL, device_id TEXT NOT NULL, device_name TEXT NOT NULL,
			remote_address TEXT NOT NULL, filename TEXT NOT NULL, path TEXT NOT NULL, source_paths TEXT NOT NULL,
			size INTEGER NOT NULL, checksum TEXT NOT NULL, status TEXT NOT NULL, progress INTEGER NOT NULL,
			speed INTEGER NOT NULL, eta_seconds INTEGER NOT NULL, started_at INTEGER, finished_at INTEGER,
			created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, error TEXT NOT NULL, attempts INTEGER NOT NULL,
			priority INTEGER NOT NULL, approved INTEGER NOT NULL, approval_required INTEGER NOT NULL,
			conflict_policy TEXT NOT NULL, chunk_size INTEGER NOT NULL, compression INTEGER NOT NULL,
			protocol_version INTEGER NOT NULL, session_token TEXT NOT NULL, session_token_hash TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS transfer_files (
			transfer_id TEXT NOT NULL, file_id TEXT NOT NULL, relative_path TEXT NOT NULL, source_path TEXT NOT NULL,
			destination_path TEXT NOT NULL, directory INTEGER NOT NULL DEFAULT 0, size INTEGER NOT NULL,
			checksum TEXT NOT NULL, chunk_size INTEGER NOT NULL, chunk_count INTEGER NOT NULL,
			PRIMARY KEY (transfer_id,file_id), FOREIGN KEY (transfer_id) REFERENCES transfers(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS chunks (
			transfer_id TEXT NOT NULL, file_id TEXT NOT NULL, chunk_index INTEGER NOT NULL, offset_bytes INTEGER NOT NULL,
			size INTEGER NOT NULL, checksum TEXT NOT NULL, status TEXT NOT NULL, attempts INTEGER NOT NULL, updated_at INTEGER NOT NULL,
			PRIMARY KEY (transfer_id,file_id,chunk_index), FOREIGN KEY (transfer_id,file_id) REFERENCES transfer_files(transfer_id,file_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS queued_transfers (transfer_id TEXT PRIMARY KEY, priority INTEGER NOT NULL, queued_at INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS transfer_history (transfer_id TEXT PRIMARY KEY, status TEXT NOT NULL, finished_at INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS failed_transfers (transfer_id TEXT PRIMARY KEY, error TEXT NOT NULL, failed_at INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS paused_transfers (transfer_id TEXT PRIMARY KEY, paused_at INTEGER NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_transfers_status_priority ON transfers(status,priority,created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_transfer_status ON chunks(transfer_id,status)`,
	}},
	{version: 3, name: "cryptographic trust metadata", apply: func(ctx context.Context, tx *sql.Tx) error {
		columns := map[string]string{
			"local_name": `TEXT NOT NULL DEFAULT ''`, "public_key": `TEXT NOT NULL DEFAULT ''`, "fingerprint": `TEXT NOT NULL DEFAULT ''`,
			"last_authenticated": `INTEGER NOT NULL DEFAULT 0`, "blocked": `INTEGER NOT NULL DEFAULT 0`,
			"identity_key_changed": `INTEGER NOT NULL DEFAULT 0`, "notes": `TEXT NOT NULL DEFAULT ''`,
		}
		names := make([]string, 0, len(columns))
		for name := range columns {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := ensureColumn(ctx, tx, "trusted_devices", name, columns[name]); err != nil {
				return err
			}
		}
		return nil
	}},
	{version: 4, name: "transfer peer identity details", apply: func(ctx context.Context, tx *sql.Tx) error {
		for _, column := range []struct {
			name       string
			definition string
		}{
			{name: "device_hostname", definition: `TEXT NOT NULL DEFAULT ''`},
			{name: "device_platform", definition: `TEXT NOT NULL DEFAULT ''`},
		} {
			if err := ensureColumn(ctx, tx, "transfers", column.name, column.definition); err != nil {
				return err
			}
		}
		return nil
	}},
}

func Migrate(ctx context.Context, connection *sql.DB) error {
	if connection == nil {
		return errors.New("database connection is required")
	}
	for _, statement := range []string{`PRAGMA busy_timeout=5000`, `PRAGMA journal_mode=WAL`, `PRAGMA foreign_keys=ON`, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at INTEGER NOT NULL DEFAULT (strftime('%s','now') * 1000))`} {
		if _, err := connection.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("prepare database migrations: %w", err)
		}
	}
	for _, item := range migrations {
		var applied int
		if err := connection.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, item.version).Scan(&applied); err != nil {
			return err
		}
		if applied != 0 {
			continue
		}
		tx, err := connection.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		failed := false
		for _, statement := range item.statements {
			if _, err = tx.ExecContext(ctx, statement); err != nil {
				failed = true
				break
			}
		}
		if !failed && item.apply != nil {
			err = item.apply(ctx, tx)
			failed = err != nil
		}
		if !failed {
			_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name) VALUES(?,?)`, item.version, item.name)
			failed = err != nil
		}
		if failed {
			_ = tx.Rollback()
			return fmt.Errorf("apply schema migration %d (%s): %w", item.version, item.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit schema migration %d: %w", item.version, err)
		}
	}
	return nil
}

func Version(ctx context.Context, connection *sql.DB) (int, error) {
	var version int
	err := connection.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version)
	return version, err
}

func ensureColumn(ctx context.Context, tx *sql.Tx, table, name, definition string) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid int
		var column, kind string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &column, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if column == name {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = tx.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+name+` `+definition)
	return err
}
