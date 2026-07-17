package transfer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/services"
)

type testPeers struct{ devices []models.Device }

func (p testPeers) Devices() []models.Device { return append([]models.Device(nil), p.devices...) }

type testAuthorizer struct{ trusted map[string]bool }

var testSharedKey = bytes.Repeat([]byte{0x53}, 32)

func (a testAuthorizer) IsTrusted(_ context.Context, id string) (bool, error) {
	return a.trusted[id], nil
}
func (a testAuthorizer) SharedKey(_ context.Context, id string) ([]byte, error) {
	if !a.trusted[id] {
		return nil, ErrUnauthorized
	}
	return append([]byte(nil), testSharedKey...), nil
}
func (a testAuthorizer) PublicKey(context.Context, string) (ed25519.PublicKey, error) {
	return make(ed25519.PublicKey, ed25519.PublicKeySize), nil
}

func testOfferAuthentication(offer Offer) PeerAuthentication {
	timestamp, nonce := strconv.FormatInt(time.Now().UTC().Unix(), 10), uuid.NewString()
	encoded, _ := json.Marshal(offer)
	return PeerAuthentication{DeviceID: offer.DeviceID, Timestamp: timestamp, Nonce: nonce, Signature: offerSignature(testSharedKey, http.MethodPost, "/v1/transfers/offers", offer.DeviceID, timestamp, nonce, encoded)}
}

type recordingPublisher struct {
	mu     sync.Mutex
	events []Event
}

func (p *recordingPublisher) Publish(event Event) {
	p.mu.Lock()
	p.events = append(p.events, event)
	p.mu.Unlock()
}

func (p *recordingPublisher) assertProgressMonotonic(t *testing.T) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	var previous int64
	for _, event := range p.events {
		if event.Type != EventProgress {
			continue
		}
		if event.Transfer.Progress < previous {
			t.Fatalf("progress moved backward from %d to %d", previous, event.Transfer.Progress)
		}
		previous = event.Transfer.Progress
	}
}

func newReceiverService(t *testing.T, senderID string) (*Service, *SQLiteStore) {
	t.Helper()
	store, database := newSQLiteStoreForTest(t)
	t.Cleanup(func() { database.Close() })
	identity, err := services.NewFileIdentityStore(filepath.Join(t.TempDir(), "identity.json")).LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	identity.Name, identity.Platform = "Receiver", "test"
	peer := models.Device{ID: senderID, Name: "Sender", Type: "desktop", Platform: "test", LocalIP: "127.0.0.1", Port: 8384, AppVersion: "test", Online: true, TransferCapability: true, SupportedProtocolVersion: 1, MaximumChunkSize: MaximumChunkSize}
	service, err := NewService(ServiceConfig{Store: store, Peers: testPeers{[]models.Device{peer}}, Authorizer: testAuthorizer{map[string]bool{senderID: true}}, Identity: identity, DataDirectory: t.TempDir(), ChunkSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	return service, store
}

func TestInboundTransferApprovalOutOfOrderChunksVerificationAndFinalize(t *testing.T) {
	senderID := uuid.NewString()
	service, _ := newReceiverService(t, senderID)
	contents := []byte("reliable local transfer")
	fileID := uuid.NewString()
	offer := Offer{TransferID: uuid.NewString(), DeviceID: senderID, DeviceName: "Sender", SessionToken: "01234567890123456789012345678901", Filename: "folder", Size: int64(len(contents)), ChunkSize: 5, ProtocolVersion: 1, Files: []File{{ID: uuid.NewString(), RelativePath: "folder", Directory: true, ChunkSize: 5}, {ID: fileID, RelativePath: "folder/message.txt", Size: int64(len(contents)), Checksum: checksumBytes(contents), ChunkSize: 5, ChunkCount: chunkCount(int64(len(contents)), 5)}}}
	received, err := service.ReceiveOffer(context.Background(), offer, "127.0.0.1", testOfferAuthentication(offer))
	if err != nil {
		t.Fatal(err)
	}
	if received.Approved || received.Status != StatusQueued {
		t.Fatalf("unexpected offer state: %#v", received)
	}
	destination := t.TempDir()
	accepted, err := service.Accept(context.Background(), offer.TransferID, AcceptRequest{DestinationPath: destination, ConflictPolicy: ConflictRename})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != StatusReceiving {
		t.Fatalf("status=%s", accepted.Status)
	}
	for index := offer.Files[1].ChunkCount - 1; index >= 0; index-- {
		start := index * 5
		end := start + 5
		if end > int64(len(contents)) {
			end = int64(len(contents))
		}
		chunk := contents[start:end]
		if _, err := service.ReceiveChunk(context.Background(), offer.TransferID, fileID, offer.SessionToken, index, checksumBytes(chunk), "", bytes.NewReader(chunk)); err != nil {
			t.Fatalf("chunk %d: %v", index, err)
		}
	}
	completed, err := service.Finalize(context.Background(), offer.TransferID, offer.SessionToken)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != StatusCompleted || completed.Progress != int64(len(contents)) {
		t.Fatalf("unexpected completed state: %#v", completed)
	}
	saved, err := os.ReadFile(filepath.Join(destination, "folder", "message.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, contents) {
		t.Fatalf("saved contents differ: %q", saved)
	}
}

func TestCorruptedChunkIsRejectedAndPartialTransferCanRetry(t *testing.T) {
	senderID := uuid.NewString()
	service, _ := newReceiverService(t, senderID)
	contents := []byte("hello")
	fileID := uuid.NewString()
	offer := Offer{TransferID: uuid.NewString(), DeviceID: senderID, DeviceName: "Sender", SessionToken: "01234567890123456789012345678901", Filename: "hello.txt", Size: 5, ChunkSize: 5, ProtocolVersion: 1, Files: []File{{ID: fileID, RelativePath: "hello.txt", Size: 5, Checksum: checksumBytes(contents), ChunkSize: 5, ChunkCount: 1}}}
	if _, err := service.ReceiveOffer(context.Background(), offer, "127.0.0.1", testOfferAuthentication(offer)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Accept(context.Background(), offer.TransferID, AcceptRequest{DestinationPath: t.TempDir(), ConflictPolicy: ConflictRename}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReceiveChunk(context.Background(), offer.TransferID, fileID, offer.SessionToken, 0, checksumBytes(contents), "", bytes.NewReader([]byte("jello"))); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	state, err := service.ProtocolStatus(context.Background(), offer.TransferID, offer.SessionToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Chunks[fileID]) != 0 {
		t.Fatalf("corrupt chunk was persisted: %#v", state)
	}
	if _, err := service.ReceiveChunk(context.Background(), offer.TransferID, fileID, offer.SessionToken, 0, checksumBytes(contents), "", bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
}

func TestFinalChecksumMismatchFailsAndPendingTransferCanBeRejected(t *testing.T) {
	senderID := uuid.NewString()
	service, _ := newReceiverService(t, senderID)
	contents := []byte("hello")
	offer := Offer{TransferID: uuid.NewString(), DeviceID: senderID, DeviceName: "Sender", SessionToken: "01234567890123456789012345678901", Filename: "hello.txt", Size: 5, ChunkSize: 5, ProtocolVersion: 1, Files: []File{{ID: uuid.NewString(), RelativePath: "hello.txt", Size: 5, Checksum: checksumBytes([]byte("other")), ChunkSize: 5, ChunkCount: 1}}}
	if _, err := service.ReceiveOffer(context.Background(), offer, "127.0.0.1", testOfferAuthentication(offer)); err != nil {
		t.Fatal(err)
	}
	rejected, err := service.Reject(context.Background(), offer.TransferID)
	if err != nil || rejected.Status != StatusCancelled {
		t.Fatalf("rejected=%#v err=%v", rejected, err)
	}
	offer.TransferID, offer.Files[0].ID = uuid.NewString(), uuid.NewString()
	if _, err = service.ReceiveOffer(context.Background(), offer, "127.0.0.1", testOfferAuthentication(offer)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Accept(context.Background(), offer.TransferID, AcceptRequest{DestinationPath: t.TempDir(), ConflictPolicy: ConflictRename}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ReceiveChunk(context.Background(), offer.TransferID, offer.Files[0].ID, offer.SessionToken, 0, checksumBytes(contents), "", bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Finalize(context.Background(), offer.TransferID, offer.SessionToken); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected final checksum mismatch, got %v", err)
	}
}

func TestPauseResumeCancelAndFailedRetryStateMachine(t *testing.T) {
	senderID := uuid.NewString()
	service, store := newReceiverService(t, senderID)
	now := time.Now().UTC()
	item := Transfer{ID: uuid.NewString(), Direction: DirectionInbound, DeviceID: senderID, Filename: "file", Status: StatusReceiving, Approved: true, CreatedAt: now, UpdatedAt: now, ConflictPolicy: ConflictRename, ChunkSize: 5, ProtocolVersion: 1}
	if err := store.SaveTransfer(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	paused, err := service.Pause(context.Background(), item.ID)
	if err != nil || paused.Status != StatusPaused {
		t.Fatalf("pause=%#v err=%v", paused, err)
	}
	resumed, err := service.Resume(context.Background(), item.ID)
	if err != nil || resumed.Status != StatusReceiving {
		t.Fatalf("resume=%#v err=%v", resumed, err)
	}
	cancelled, err := service.Cancel(context.Background(), item.ID)
	if err != nil || cancelled.Status != StatusCancelled {
		t.Fatalf("cancel=%#v err=%v", cancelled, err)
	}
	item.ID = uuid.NewString()
	item.Status = StatusFailed
	item.Error = "disconnect"
	if err := store.SaveTransfer(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	retried, err := service.Retry(context.Background(), item.ID)
	if err != nil || retried.Status != StatusReceiving || retried.Error != "" {
		t.Fatalf("retry=%#v err=%v", retried, err)
	}
}

func TestOfferRejectsUntrustedSpoofedAndUnsupportedPeers(t *testing.T) {
	senderID := uuid.NewString()
	service, _ := newReceiverService(t, senderID)
	offer := Offer{TransferID: uuid.NewString(), DeviceID: uuid.NewString(), DeviceName: "Unknown", SessionToken: "01234567890123456789012345678901", Filename: "x", ChunkSize: 4, ProtocolVersion: 1, Files: []File{{ID: uuid.NewString(), RelativePath: "x", Checksum: checksumBytes(nil), ChunkSize: 4}}}
	if _, err := service.ReceiveOffer(context.Background(), offer, "127.0.0.1", testOfferAuthentication(offer)); !errors.Is(err, ErrUntrustedDevice) {
		t.Fatalf("expected untrusted error, got %v", err)
	}
	offer.DeviceID = senderID
	if _, err := service.ReceiveOffer(context.Background(), offer, "192.168.1.10", testOfferAuthentication(offer)); !errors.Is(err, ErrUntrustedDevice) {
		t.Fatalf("expected spoof protection, got %v", err)
	}
}

func TestSenderAndReceiverProtocolStreamsFolderWithCompression(t *testing.T) {
	senderID := uuid.NewString()
	receiver, _ := newReceiverService(t, senderID)
	receiverEvents := &recordingPublisher{}
	receiver.publisher = receiverEvents
	destination := t.TempDir()
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		parts := strings.Split(request.URL.Path, "/")
		token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		var value any
		var err error
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v1/transfers/offers":
			var offer Offer
			err = json.NewDecoder(request.Body).Decode(&offer)
			if err == nil {
				auth := PeerAuthentication{DeviceID: request.Header.Get("X-SyncSpace-Device-ID"), Timestamp: request.Header.Get("X-SyncSpace-Timestamp"), Nonce: request.Header.Get("X-SyncSpace-Nonce"), Signature: request.Header.Get("X-SyncSpace-Signature")}
				_, err = receiver.ReceiveOffer(request.Context(), offer, "127.0.0.1", auth)
			}
			if err == nil {
				_, err = receiver.Accept(request.Context(), offer.TransferID, AcceptRequest{DestinationPath: destination, ConflictPolicy: ConflictRename})
			}
			value = OfferResponse{TransferID: offer.TransferID, Status: StatusReceiving, Approved: true}
		case request.Method == http.MethodGet && len(parts) == 5 && parts[4] == "status":
			value, err = receiver.ProtocolStatus(request.Context(), parts[3], token)
		case request.Method == http.MethodPut && len(parts) == 8 && parts[4] == "files" && parts[6] == "chunks":
			index, parseErr := strconv.ParseInt(parts[7], 10, 64)
			if parseErr != nil {
				err = parseErr
				break
			}
			value, err = receiver.ReceiveChunk(request.Context(), parts[3], parts[5], token, index, request.Header.Get("X-Chunk-SHA256"), request.Header.Get("Content-Encoding"), request.Body)
		case request.Method == http.MethodPost && len(parts) == 5 && parts[4] == "complete":
			value, err = receiver.Finalize(request.Context(), parts[3], token)
		default:
			http.NotFound(response, request)
			return
		}
		if err != nil {
			http.Error(response, err.Error(), http.StatusConflict)
			return
		}
		if value == nil {
			value = map[string]string{"status": "ok"}
		}
		_ = json.NewEncoder(response).Encode(value)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	sourceRoot := t.TempDir()
	folder := filepath.Join(sourceRoot, "project")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	contents := bytes.Repeat([]byte("compressible local data\n"), 512*1024)
	if err := os.WriteFile(filepath.Join(folder, "readme.txt"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	loose := filepath.Join(sourceRoot, "tiny.txt")
	if err := os.WriteFile(loose, []byte("tiny file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	senderStore, senderDB := newSQLiteStoreForTest(t)
	defer senderDB.Close()
	receiverPeer := models.Device{ID: receiver.identity.ID, Name: "Receiver", Type: "desktop", Platform: "test", LocalIP: "127.0.0.1", Port: port, AppVersion: "test", Online: true, TransferCapability: true, SupportedProtocolVersion: 1, MaximumChunkSize: 256 * 1024, CompressionSupport: true}
	senderIdentity, err := services.NewFileIdentityStore(filepath.Join(t.TempDir(), "sender-identity.json")).LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	senderIdentity.ID, senderIdentity.Name, senderIdentity.Platform = senderID, "Sender", "test"
	senderEvents := &recordingPublisher{}
	sender, err := NewService(ServiceConfig{Store: senderStore, Peers: testPeers{[]models.Device{receiverPeer}}, Authorizer: testAuthorizer{map[string]bool{receiver.identity.ID: true}}, Identity: senderIdentity, DataDirectory: t.TempDir(), ChunkSize: 256 * 1024, HTTPClient: server.Client(), Publisher: senderEvents})
	if err != nil {
		t.Fatal(err)
	}
	queued, err := sender.Queue(context.Background(), QueueRequest{DeviceID: receiver.identity.ID, Paths: []string{folder, loose}, ConflictPolicy: ConflictRename})
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.processOutbound(context.Background(), queued.ID); err != nil {
		t.Fatal(err)
	}
	sent, err := sender.Get(context.Background(), queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Status != StatusCompleted {
		t.Fatalf("sender status=%s", sent.Status)
	}
	saved, err := os.ReadFile(filepath.Join(destination, "project", "readme.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, contents) {
		t.Fatal("end-to-end contents differ")
	}
	tiny, err := os.ReadFile(filepath.Join(destination, "tiny.txt"))
	if err != nil || string(tiny) != "tiny file\n" {
		t.Fatalf("second file=%q err=%v", tiny, err)
	}
	senderEvents.assertProgressMonotonic(t)
	receiverEvents.assertProgressMonotonic(t)
}

func TestRecoveryRequeuesInterruptedOutboundAndPreservesInboundPartialState(t *testing.T) {
	senderID := uuid.NewString()
	service, store := newReceiverService(t, senderID)
	now := time.Now().UTC()
	outbound := Transfer{ID: uuid.NewString(), Direction: DirectionOutbound, DeviceID: senderID, Filename: "out", Status: StatusSending, CreatedAt: now, UpdatedAt: now, ConflictPolicy: ConflictRename, ChunkSize: 4, ProtocolVersion: 1}
	inbound := Transfer{ID: uuid.NewString(), Direction: DirectionInbound, DeviceID: senderID, Filename: "in", Status: StatusVerifying, Approved: true, CreatedAt: now, UpdatedAt: now, ConflictPolicy: ConflictRename, ChunkSize: 4, ProtocolVersion: 1}
	if err := store.SaveTransfer(context.Background(), outbound); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTransfer(context.Background(), inbound); err != nil {
		t.Fatal(err)
	}
	if err := service.recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	outbound, _ = store.GetTransfer(context.Background(), outbound.ID)
	inbound, _ = store.GetTransfer(context.Background(), inbound.ID)
	if outbound.Status != StatusQueued || inbound.Status != StatusReceiving {
		t.Fatalf("recovery states: outbound=%s inbound=%s", outbound.Status, inbound.Status)
	}
}
