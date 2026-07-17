package transfer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appdatabase "github.com/louisboii747/syncspace/backend/internal/database"
	_ "modernc.org/sqlite"
)

// SQLiteStore persists transfers in the shared SyncSpace database. The mirror
// tables intentionally make queue/history/failure recovery inspectable without
// deriving every view from a status string.
type SQLiteStore struct{ database *sql.DB }

func NewSQLiteStore(ctx context.Context, database *sql.DB) (*SQLiteStore, error) {
	if database == nil {
		return nil, errors.New("database is required")
	}
	s := &SQLiteStore{database: database}
	if err := s.migrate(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	return appdatabase.Migrate(ctx, s.database)
}

func (s *SQLiteStore) SaveTransfer(ctx context.Context, transfer Transfer) error {
	paths, err := json.Marshal(transfer.SourcePaths)
	if err != nil {
		return fmt.Errorf("encode source paths: %w", err)
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transfer save: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO transfers (
		id,direction,device_id,device_name,remote_address,filename,path,source_paths,size,checksum,
		status,progress,speed,eta_seconds,started_at,finished_at,created_at,updated_at,error,attempts,
		priority,approved,approval_required,conflict_policy,chunk_size,compression,protocol_version,session_token,session_token_hash)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET direction=excluded.direction,device_id=excluded.device_id,
		device_name=excluded.device_name,remote_address=excluded.remote_address,filename=excluded.filename,
		path=excluded.path,source_paths=excluded.source_paths,size=excluded.size,checksum=excluded.checksum,
		status=excluded.status,progress=excluded.progress,speed=excluded.speed,eta_seconds=excluded.eta_seconds,
		started_at=excluded.started_at,finished_at=excluded.finished_at,updated_at=excluded.updated_at,
		error=excluded.error,attempts=excluded.attempts,priority=excluded.priority,approved=excluded.approved,
		approval_required=excluded.approval_required,conflict_policy=excluded.conflict_policy,
		chunk_size=excluded.chunk_size,compression=excluded.compression,protocol_version=excluded.protocol_version,
		session_token=excluded.session_token,session_token_hash=excluded.session_token_hash`,
		transfer.ID, transfer.Direction, transfer.DeviceID, transfer.DeviceName, transfer.RemoteAddress,
		transfer.Filename, transfer.Path, string(paths), transfer.Size, transfer.Checksum, transfer.Status,
		transfer.Progress, transfer.Speed, transfer.ETASeconds, nullableTime(transfer.StartedAt),
		nullableTime(transfer.FinishedAt), transfer.CreatedAt.UnixMilli(), transfer.UpdatedAt.UnixMilli(),
		transfer.Error, transfer.Attempts, transfer.Priority, transfer.Approved, transfer.ApprovalRequired,
		transfer.ConflictPolicy, transfer.ChunkSize, transfer.Compression, transfer.ProtocolVersion,
		transfer.SessionToken, transfer.SessionTokenHash)
	if err != nil {
		return fmt.Errorf("save transfer: %w", err)
	}
	for _, file := range transfer.Files {
		_, err = tx.ExecContext(ctx, `INSERT INTO transfer_files
			(transfer_id,file_id,relative_path,source_path,destination_path,directory,size,checksum,chunk_size,chunk_count)
			VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT(transfer_id,file_id) DO UPDATE SET
			relative_path=excluded.relative_path,source_path=excluded.source_path,
			destination_path=excluded.destination_path,directory=excluded.directory,size=excluded.size,checksum=excluded.checksum,
			chunk_size=excluded.chunk_size,chunk_count=excluded.chunk_count`, transfer.ID, file.ID,
			file.RelativePath, file.SourcePath, file.DestinationPath, file.Directory, file.Size, file.Checksum,
			file.ChunkSize, file.ChunkCount)
		if err != nil {
			return fmt.Errorf("save transfer file: %w", err)
		}
	}
	for _, table := range []string{"queued_transfers", "transfer_history", "failed_transfers", "paused_transfers"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE transfer_id = ?`, transfer.ID); err != nil {
			return err
		}
	}
	switch transfer.Status {
	case StatusQueued, StatusPreparing, StatusConnecting, StatusNegotiating, StatusResuming:
		_, err = tx.ExecContext(ctx, `INSERT INTO queued_transfers(transfer_id,priority,queued_at) VALUES(?,?,?)`, transfer.ID, transfer.Priority, transfer.UpdatedAt.UnixMilli())
	case StatusCompleted, StatusCancelled:
		finished := transfer.UpdatedAt.UnixMilli()
		if transfer.FinishedAt != nil {
			finished = transfer.FinishedAt.UnixMilli()
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO transfer_history(transfer_id,status,finished_at) VALUES(?,?,?)`, transfer.ID, transfer.Status, finished)
	case StatusFailed:
		_, err = tx.ExecContext(ctx, `INSERT INTO failed_transfers(transfer_id,error,failed_at) VALUES(?,?,?)`, transfer.ID, transfer.Error, transfer.UpdatedAt.UnixMilli())
	case StatusPaused:
		_, err = tx.ExecContext(ctx, `INSERT INTO paused_transfers(transfer_id,paused_at) VALUES(?,?)`, transfer.ID, transfer.UpdatedAt.UnixMilli())
	}
	if err != nil {
		return fmt.Errorf("save transfer state mirror: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transfer save: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetTransfer(ctx context.Context, id string) (Transfer, error) {
	row := s.database.QueryRowContext(ctx, transferSelect+` WHERE id = ?`, id)
	transfer, err := scanTransfer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Transfer{}, ErrNotFound
	}
	if err != nil {
		return Transfer{}, err
	}
	transfer.Files, err = s.listFiles(ctx, id)
	return transfer, err
}

func (s *SQLiteStore) ListTransfers(ctx context.Context) ([]Transfer, error) {
	rows, err := s.database.QueryContext(ctx, transferSelect+` ORDER BY priority ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list transfers: %w", err)
	}
	defer rows.Close()
	result := make([]Transfer, 0)
	for rows.Next() {
		item, err := scanTransfer(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Files, err = s.listFiles(ctx, result[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *SQLiteStore) DeleteHistory(ctx context.Context) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM chunks WHERE transfer_id IN (SELECT id FROM transfers WHERE status IN (?,?))`, StatusCompleted, StatusCancelled); err != nil {
		return fmt.Errorf("delete history chunks: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM transfer_files WHERE transfer_id IN (SELECT id FROM transfers WHERE status IN (?,?))`, StatusCompleted, StatusCancelled); err != nil {
		return fmt.Errorf("delete history files: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM transfers WHERE status IN (?,?)`, StatusCompleted, StatusCancelled); err != nil {
		return fmt.Errorf("delete transfer history: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM transfer_history`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) SaveChunk(ctx context.Context, chunk Chunk) error {
	_, err := s.database.ExecContext(ctx, `INSERT INTO chunks
		(transfer_id,file_id,chunk_index,offset_bytes,size,checksum,status,attempts,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(transfer_id,file_id,chunk_index) DO UPDATE SET
		offset_bytes=excluded.offset_bytes,size=excluded.size,checksum=excluded.checksum,
		status=excluded.status,attempts=excluded.attempts,updated_at=excluded.updated_at`,
		chunk.TransferID, chunk.FileID, chunk.Index, chunk.Offset, chunk.Size, chunk.Checksum,
		chunk.Status, chunk.Attempts, chunk.UpdatedAt.UnixMilli())
	if err != nil {
		return fmt.Errorf("save chunk: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListChunks(ctx context.Context, transferID string) ([]Chunk, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT transfer_id,file_id,chunk_index,offset_bytes,size,checksum,status,attempts,updated_at FROM chunks WHERE transfer_id=? ORDER BY file_id,chunk_index`, transferID)
	if err != nil {
		return nil, fmt.Errorf("list chunks: %w", err)
	}
	defer rows.Close()
	result := make([]Chunk, 0)
	for rows.Next() {
		var c Chunk
		var updated int64
		if err := rows.Scan(&c.TransferID, &c.FileID, &c.Index, &c.Offset, &c.Size, &c.Checksum, &c.Status, &c.Attempts, &updated); err != nil {
			return nil, err
		}
		c.UpdatedAt = time.UnixMilli(updated).UTC()
		result = append(result, c)
	}
	return result, rows.Err()
}

const transferSelect = `SELECT id,direction,device_id,device_name,remote_address,filename,path,source_paths,size,checksum,status,progress,speed,eta_seconds,started_at,finished_at,created_at,updated_at,error,attempts,priority,approved,approval_required,conflict_policy,chunk_size,compression,protocol_version,session_token,session_token_hash FROM transfers`

type scanner interface{ Scan(...any) error }

func scanTransfer(row scanner) (Transfer, error) {
	var t Transfer
	var paths string
	var started, finished sql.NullInt64
	var created, updated int64
	err := row.Scan(&t.ID, &t.Direction, &t.DeviceID, &t.DeviceName, &t.RemoteAddress, &t.Filename, &t.Path, &paths, &t.Size, &t.Checksum, &t.Status, &t.Progress, &t.Speed, &t.ETASeconds, &started, &finished, &created, &updated, &t.Error, &t.Attempts, &t.Priority, &t.Approved, &t.ApprovalRequired, &t.ConflictPolicy, &t.ChunkSize, &t.Compression, &t.ProtocolVersion, &t.SessionToken, &t.SessionTokenHash)
	if err != nil {
		return Transfer{}, err
	}
	if err := json.Unmarshal([]byte(paths), &t.SourcePaths); err != nil {
		return Transfer{}, fmt.Errorf("decode source paths: %w", err)
	}
	t.CreatedAt = time.UnixMilli(created).UTC()
	t.UpdatedAt = time.UnixMilli(updated).UTC()
	if started.Valid {
		value := time.UnixMilli(started.Int64).UTC()
		t.StartedAt = &value
	}
	if finished.Valid {
		value := time.UnixMilli(finished.Int64).UTC()
		t.FinishedAt = &value
	}
	return t, nil
}

func (s *SQLiteStore) listFiles(ctx context.Context, transferID string) ([]File, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT file_id,relative_path,source_path,destination_path,directory,size,checksum,chunk_size,chunk_count FROM transfer_files WHERE transfer_id=? ORDER BY relative_path`, transferID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]File, 0)
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.RelativePath, &f.SourcePath, &f.DestinationPath, &f.Directory, &f.Size, &f.Checksum, &f.ChunkSize, &f.ChunkCount); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().UnixMilli()
}
