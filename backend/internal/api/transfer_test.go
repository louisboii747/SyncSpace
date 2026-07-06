package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/transfer"
)

type fakeTransferService struct {
	queued  transfer.QueueRequest
	offered transfer.Offer
	chunk   []byte
	staged  []byte
	stageID string
}

func (s *fakeTransferService) Queue(_ context.Context, r transfer.QueueRequest) (transfer.Transfer, error) {
	s.queued = r
	return transfer.Transfer{ID: "queued", Status: transfer.StatusQueued}, nil
}
func (s *fakeTransferService) ReceiveOffer(_ context.Context, o transfer.Offer, _ string) (transfer.Transfer, error) {
	s.offered = o
	return transfer.Transfer{ID: o.TransferID, Status: transfer.StatusQueued}, nil
}
func (s *fakeTransferService) List(context.Context) ([]transfer.Transfer, error) {
	return []transfer.Transfer{{ID: "one"}}, nil
}
func (s *fakeTransferService) Get(_ context.Context, id string) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, Status: transfer.StatusSending}, nil
}
func (s *fakeTransferService) DeleteHistory(context.Context) error { return nil }
func (s *fakeTransferService) Accept(_ context.Context, id string, _ transfer.AcceptRequest) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, Status: transfer.StatusReceiving}, nil
}
func (s *fakeTransferService) Reject(_ context.Context, id string) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, Status: transfer.StatusCancelled}, nil
}
func (s *fakeTransferService) Pause(_ context.Context, id string) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, Status: transfer.StatusPaused}, nil
}
func (s *fakeTransferService) Resume(_ context.Context, id string) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, Status: transfer.StatusResuming}, nil
}
func (s *fakeTransferService) Cancel(_ context.Context, id string) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, Status: transfer.StatusCancelled}, nil
}
func (s *fakeTransferService) Retry(_ context.Context, id string) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, Status: transfer.StatusQueued}, nil
}
func (s *fakeTransferService) ProtocolStatus(_ context.Context, id, token string) (transfer.ResumeMap, error) {
	if token != "secret" {
		return transfer.ResumeMap{}, transfer.ErrUnauthorized
	}
	return transfer.ResumeMap{TransferID: id, Approved: true}, nil
}
func (s *fakeTransferService) ProtocolCancel(_ context.Context, id, token string) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, Status: transfer.StatusCancelled}, nil
}
func (s *fakeTransferService) ReceiveChunk(_ context.Context, id, fileID, token string, index int64, checksum, encoding string, body io.Reader) (transfer.Transfer, error) {
	s.chunk, _ = io.ReadAll(body)
	return transfer.Transfer{ID: id, Progress: int64(len(s.chunk))}, nil
}
func (s *fakeTransferService) Finalize(_ context.Context, id, token string) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, Status: transfer.StatusCompleted}, nil
}
func (s *fakeTransferService) CreateStage(context.Context) (transfer.StagingSession, error) {
	s.stageID = "f5c85f05-c932-4da6-934c-489438086270"
	return transfer.StagingSession{ID: s.stageID}, nil
}
func (s *fakeTransferService) UploadStagedFile(_ context.Context, id, path string, size int64, body io.Reader) error {
	s.staged, _ = io.ReadAll(body)
	return nil
}
func (s *fakeTransferService) QueueStage(_ context.Context, id string, request transfer.StageQueueRequest) (transfer.Transfer, error) {
	return transfer.Transfer{ID: id, DeviceID: request.DeviceID, Status: transfer.StatusQueued}, nil
}
func (s *fakeTransferService) DeleteStage(context.Context, string) error { return nil }

func newTransferRouter(service TransferService) *gin.Engine {
	return NewRouter(RouterConfig{Discovery: &fakeDiscoveryService{}, DiscoverySocket: func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) }, Pairing: &fakePairingService{}, PairingSocket: func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) }, Transfer: service, TransferSocket: func(c *gin.Context) { c.Status(http.StatusSwitchingProtocols) }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}
func localRequest(method, path string, body io.Reader) *http.Request {
	request := httptest.NewRequest(method, path, body)
	request.RemoteAddr = "127.0.0.1:5000"
	return request
}

func TestTransferManagementRoutesAreLoopbackOnly(t *testing.T) {
	service := &fakeTransferService{}
	router := newTransferRouter(service)
	body := bytes.NewBufferString(`{"deviceId":"device","paths":["file"]}`)
	request := localRequest(http.MethodPost, "/transfers", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || len(service.queued.Paths) != 1 {
		t.Fatalf("queue status=%d body=%s", response.Code, response.Body.String())
	}
	remote := httptest.NewRequest(http.MethodGet, "/transfers", nil)
	remote.RemoteAddr = "192.168.1.20:5000"
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, remote)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("remote management status=%d", denied.Code)
	}
}

func TestPeerProtocolOfferStatusChunkAndFinalize(t *testing.T) {
	service := &fakeTransferService{}
	router := newTransferRouter(service)
	offer := transfer.Offer{TransferID: "transfer", DeviceID: "device", DeviceName: "Peer", SessionToken: "token", Filename: "file", ProtocolVersion: 1}
	contents, _ := json.Marshal(offer)
	offerRequest := httptest.NewRequest(http.MethodPost, "/v1/transfers/offers", bytes.NewReader(contents))
	offerRequest.Header.Set("Content-Type", "application/json")
	offerResponse := httptest.NewRecorder()
	router.ServeHTTP(offerResponse, offerRequest)
	if offerResponse.Code != http.StatusAccepted || service.offered.TransferID != "transfer" {
		t.Fatalf("offer status=%d body=%s", offerResponse.Code, offerResponse.Body.String())
	}
	statusRequest := httptest.NewRequest(http.MethodGet, "/v1/transfers/transfer/status", nil)
	statusRequest.Header.Set("Authorization", "Bearer secret")
	statusResponse := httptest.NewRecorder()
	router.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("status endpoint=%d", statusResponse.Code)
	}
	chunkRequest := httptest.NewRequest(http.MethodPut, "/v1/transfers/transfer/files/file/chunks/0", bytes.NewReader([]byte("chunk")))
	chunkRequest.Header.Set("Authorization", "Bearer secret")
	chunkResponse := httptest.NewRecorder()
	router.ServeHTTP(chunkResponse, chunkRequest)
	if chunkResponse.Code != http.StatusOK || string(service.chunk) != "chunk" {
		t.Fatalf("chunk status=%d", chunkResponse.Code)
	}
	badStatus := httptest.NewRecorder()
	router.ServeHTTP(badStatus, httptest.NewRequest(http.MethodGet, "/v1/transfers/transfer/status", nil))
	if badStatus.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status=%d", badStatus.Code)
	}
}

func TestBrowserStagingRoutesStreamAndQueueLocally(t *testing.T) {
	service := &fakeTransferService{}
	router := newTransferRouter(service)
	created := httptest.NewRecorder()
	router.ServeHTTP(created, localRequest(http.MethodPost, "/transfers/staging", nil))
	if created.Code != http.StatusCreated || service.stageID == "" {
		t.Fatalf("create staging status=%d body=%s", created.Code, created.Body.String())
	}

	upload := localRequest(http.MethodPut, "/transfers/staging/"+service.stageID+"/files?path=folder%2Fnote.txt", bytes.NewReader([]byte("hello")))
	upload.ContentLength = 5
	uploaded := httptest.NewRecorder()
	router.ServeHTTP(uploaded, upload)
	if uploaded.Code != http.StatusNoContent || string(service.staged) != "hello" {
		t.Fatalf("upload staging status=%d body=%s", uploaded.Code, uploaded.Body.String())
	}

	queueBody := bytes.NewBufferString(`{"deviceId":"peer","roots":["folder"],"conflictPolicy":"rename"}`)
	queue := localRequest(http.MethodPost, "/transfers/staging/"+service.stageID+"/queue", queueBody)
	queue.Header.Set("Content-Type", "application/json")
	queued := httptest.NewRecorder()
	router.ServeHTTP(queued, queue)
	if queued.Code != http.StatusAccepted {
		t.Fatalf("queue staging status=%d body=%s", queued.Code, queued.Body.String())
	}

	remote := httptest.NewRequest(http.MethodPost, "/transfers/staging", nil)
	remote.RemoteAddr = "192.168.1.20:5000"
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, remote)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("remote staging status=%d", denied.Code)
	}
}
