package transfer

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func newSQLiteStoreForTest(t *testing.T) (*SQLiteStore, *sql.DB) {
	t.Helper()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "syncspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	store, err := NewSQLiteStore(context.Background(), database)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	return store, database
}

func TestSQLiteStorePersistsFilesChunksAndStateMirrors(t *testing.T) {
	store, database := newSQLiteStoreForTest(t)
	defer database.Close()
	now := time.Now().UTC().Truncate(time.Millisecond)
	item := Transfer{ID: "6879059d-38e1-4e4a-9a32-173cb920f7e0", Direction: DirectionOutbound, DeviceID: "771e8205-c766-4087-85a8-18a439fe09f5", DeviceName: "Studio Laptop", DeviceHostname: "studio-laptop", DevicePlatform: "linux", Filename: "data.bin", SourcePaths: []string{`C:\data.bin`}, Files: []File{{ID: "e6cd90e9-ebf3-4324-a8a9-50ffb78dd41f", RelativePath: "data.bin", SourcePath: `C:\data.bin`, Size: 8, Checksum: stringsOf('a', 64), ChunkSize: 4, ChunkCount: 2}}, Size: 8, Status: StatusQueued, CreatedAt: now, UpdatedAt: now, Priority: 1, ConflictPolicy: ConflictPrompt, ChunkSize: 4, ProtocolVersion: 1, SessionToken: "secret"}
	if err := store.SaveTransfer(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	chunk := Chunk{TransferID: item.ID, FileID: item.Files[0].ID, Index: 0, Size: 4, Checksum: stringsOf('b', 64), Status: ChunkComplete, UpdatedAt: now}
	if err := store.SaveChunk(context.Background(), chunk); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetTransfer(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Files) != 1 || loaded.SessionToken != "secret" || loaded.DeviceHostname != "studio-laptop" || loaded.DevicePlatform != "linux" {
		t.Fatalf("unexpected persisted transfer: %#v", loaded)
	}
	assertMirrorCount(t, database, "queued_transfers", 1)
	item.Status = StatusFailed
	item.Error = "network disconnected"
	item.UpdatedAt = now.Add(time.Second)
	if err := store.SaveTransfer(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	assertMirrorCount(t, database, "queued_transfers", 0)
	assertMirrorCount(t, database, "failed_transfers", 1)
	chunks, err := store.ListChunks(context.Background(), item.ID)
	if err != nil || len(chunks) != 1 {
		t.Fatalf("chunks=%#v err=%v", chunks, err)
	}
}

func TestSQLiteStoreRecoversThousandsInStableQueueOrder(t *testing.T) {
	store, database := newSQLiteStoreForTest(t)
	defer database.Close()
	now := time.Now().UTC()
	for i := 0; i < 1200; i++ {
		item := Transfer{ID: uuidFromInt(i), Direction: DirectionOutbound, DeviceID: "771e8205-c766-4087-85a8-18a439fe09f5", Filename: "file", Status: StatusQueued, CreatedAt: now, UpdatedAt: now, Priority: int64(1200 - i), ConflictPolicy: ConflictPrompt, ChunkSize: DefaultChunkSize, ProtocolVersion: 1}
		if err := store.SaveTransfer(context.Background(), item); err != nil {
			t.Fatal(err)
		}
	}
	items, err := store.ListTransfers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1200 || items[0].Priority != 1 || items[len(items)-1].Priority != 1200 {
		t.Fatalf("queue ordering or count incorrect: %d", len(items))
	}
}

func TestDeleteHistoryClearsAllTerminalRecordsAndKeepsActiveTransfers(t *testing.T) {
	store, database := newSQLiteStoreForTest(t)
	defer database.Close()
	now := time.Now().UTC()
	statuses := []Status{StatusCompleted, StatusCancelled, StatusFailed, StatusSending}
	for index, status := range statuses {
		item := Transfer{ID: uuidFromInt(index + 1), Direction: DirectionOutbound, DeviceID: "771e8205-c766-4087-85a8-18a439fe09f5", Filename: "file", Status: status, CreatedAt: now, UpdatedAt: now, Priority: int64(index), ConflictPolicy: ConflictPrompt, ChunkSize: DefaultChunkSize, ProtocolVersion: 1}
		if status == StatusFailed {
			item.Error = "test failure"
		}
		if err := store.SaveTransfer(context.Background(), item); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.DeleteHistory(context.Background()); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListTransfers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != StatusSending {
		t.Fatalf("unexpected remaining transfers: %#v", items)
	}
	assertMirrorCount(t, database, "failed_transfers", 0)
	assertMirrorCount(t, database, "transfer_history", 0)
}

func assertMirrorCount(t *testing.T, database *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := database.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s count=%d want=%d", table, got, want)
	}
}
func stringsOf(value byte, count int) string {
	result := make([]byte, count)
	for i := range result {
		result[i] = value
	}
	return string(result)
}
func uuidFromInt(value int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", value) }
