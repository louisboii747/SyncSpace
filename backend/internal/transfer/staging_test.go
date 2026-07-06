package transfer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/services"
)

func TestStagingStreamsNestedFilesAndPromotesTopLevelRoots(t *testing.T) {
	peerID := uuid.NewString()
	store, database := newSQLiteStoreForTest(t)
	defer database.Close()
	dataDirectory := t.TempDir()
	service, err := NewService(ServiceConfig{
		Store:         store,
		Peers:         testPeers{devices: []models.Device{{ID: peerID, Name: "Peer", LocalIP: "127.0.0.1", Port: 8384, Online: true, TransferCapability: true, SupportedProtocolVersion: ProtocolVersion, MaximumChunkSize: MaximumChunkSize}}},
		Authorizer:    testAuthorizer{trusted: map[string]bool{peerID: true}},
		Identity:      services.Identity{ID: uuid.NewString(), Name: "Local", Type: "desktop", Platform: "test"},
		DataDirectory: dataDirectory,
	})
	if err != nil {
		t.Fatal(err)
	}
	stage, err := service.CreateStage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	contents := []byte("hello from the browser")
	if err = service.UploadStagedFile(context.Background(), stage.ID, "project/readme.txt", int64(len(contents)), bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
	queued, err := service.QueueStage(context.Background(), stage.ID, StageQueueRequest{DeviceID: peerID, Roots: []string{"project"}, ConflictPolicy: ConflictRename})
	if err != nil {
		t.Fatal(err)
	}
	if queued.Status != StatusQueued || len(queued.SourcePaths) != 1 || filepath.Base(queued.SourcePaths[0]) != "project" {
		t.Fatalf("unexpected queued transfer: %#v", queued)
	}
	saved, err := os.ReadFile(filepath.Join(dataDirectory, "staging", "queued", stage.ID, "project", "readme.txt"))
	if err != nil || !bytes.Equal(saved, contents) {
		t.Fatalf("saved staging contents=%q err=%v", saved, err)
	}
}

func TestStagingRejectsTraversalDuplicatesAndShortBodies(t *testing.T) {
	peerID := uuid.NewString()
	service, _ := newReceiverService(t, peerID)
	stage, err := service.CreateStage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = service.UploadStagedFile(context.Background(), stage.ID, "../escape.txt", 1, bytes.NewReader([]byte("x"))); !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
	if err = service.UploadStagedFile(context.Background(), stage.ID, "short.txt", 5, bytes.NewReader([]byte("x"))); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected short body rejection, got %v", err)
	}
	if err = service.UploadStagedFile(context.Background(), stage.ID, "same.txt", 1, bytes.NewReader([]byte("x"))); err != nil {
		t.Fatal(err)
	}
	if err = service.UploadStagedFile(context.Background(), stage.ID, "same.txt", 1, bytes.NewReader([]byte("y"))); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected duplicate rejection, got %v", err)
	}
}
