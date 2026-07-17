package transfer

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	approvalPollInterval = time.Second
	chunkRetryLimit      = 3
)

func (s *Service) processOutbound(ctx context.Context, id string) error {
	t, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if len(t.Files) == 0 {
		t.Status = StatusPreparing
		t.UpdatedAt = s.now().UTC()
		if err = s.store.SaveTransfer(ctx, t); err != nil {
			return err
		}
		s.logAndPublish(EventQueueUpdated, t)
		files, size, checksum, err := buildManifest(t.SourcePaths, t.ChunkSize)
		if err != nil {
			return err
		}
		t.Files = files
		t.Size = size
		t.Checksum = checksum
		t.Attempts++
		t.UpdatedAt = s.now().UTC()
		if err = s.store.SaveTransfer(ctx, t); err != nil {
			return err
		}
	}
	peer, found := s.peer(t.DeviceID)
	if !found || !peer.Online {
		return errors.New("destination device is offline")
	}
	t.RemoteAddress = s.peerAddress(peer)
	if t.SessionToken == "" {
		t.SessionToken, err = generateToken()
		if err != nil {
			return err
		}
	}
	now := s.now().UTC()
	if t.StartedAt == nil {
		t.StartedAt = &now
	}
	t.Status = StatusConnecting
	t.UpdatedAt = now
	if err = s.store.SaveTransfer(ctx, t); err != nil {
		return err
	}
	s.logAndPublish(EventQueueUpdated, t)
	offer := Offer{TransferID: t.ID, DeviceID: s.identity.ID, DeviceName: s.identity.Name, SessionToken: t.SessionToken, Filename: t.Filename, Size: t.Size, Files: t.Files, ChunkSize: t.ChunkSize, Compression: t.Compression, ProtocolVersion: ProtocolVersion}
	t.Status = StatusNegotiating
	t.UpdatedAt = s.now().UTC()
	if err = s.store.SaveTransfer(ctx, t); err != nil {
		return err
	}
	var response OfferResponse
	if err = s.jsonRequest(ctx, http.MethodPost, t.RemoteAddress+"/v1/transfers/offers", t.DeviceID, "", offer, &response); err != nil {
		return fmt.Errorf("negotiate transfer: %w", err)
	}
	var resume ResumeMap
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		resume, err = s.remoteStatus(ctx, t)
		if err == nil && resume.Approved && (resume.Status == StatusReceiving || resume.Status == StatusResuming) {
			break
		}
		if err == nil && (resume.Status == StatusCancelled || resume.Status == StatusFailed) {
			return fmt.Errorf("receiver ended transfer with status %s", resume.Status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(approvalPollInterval):
		}
	}
	t.Status = StatusSending
	t.Approved = true
	t.UpdatedAt = s.now().UTC()
	if err = s.store.SaveTransfer(ctx, t); err != nil {
		return err
	}
	s.logAndPublish(EventResumed, t)
	completed := resume.Chunks
	type task struct {
		file  File
		index int64
	}
	tasks := make(chan task, s.chunkWorkers*2)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var firstErr error
	var errorOnce sync.Once
	for worker := 0; worker < s.chunkWorkers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range tasks {
				if err := s.sendChunk(workerCtx, t, item.file, item.index); err != nil {
					errorOnce.Do(func() { firstErr = err; cancel() })
					return
				}
				chunkSize := item.file.ChunkSize
				if remaining := item.file.Size - item.index*item.file.ChunkSize; remaining < chunkSize {
					chunkSize = remaining
				}
				chunk := Chunk{TransferID: t.ID, FileID: item.file.ID, Index: item.index, Offset: item.index * item.file.ChunkSize, Size: chunkSize, Status: ChunkComplete, Attempts: 1, UpdatedAt: s.now().UTC()}
				if err := s.store.SaveChunk(workerCtx, chunk); err != nil {
					errorOnce.Do(func() { firstErr = err; cancel() })
					return
				}
				current, getErr := s.Get(workerCtx, t.ID)
				if getErr == nil {
					_, _ = s.refreshProgress(workerCtx, current, s.now().UTC())
				}
			}
		}()
	}
enqueue:
	for _, file := range t.Files {
		if file.Directory {
			continue
		}
		known := make(map[int64]bool)
		for _, index := range completed[file.ID] {
			known[index] = true
		}
		for index := int64(0); index < file.ChunkCount; index++ {
			if known[index] {
				continue
			}
			select {
			case <-workerCtx.Done():
				break enqueue
			case tasks <- task{file: file, index: index}:
			}
		}
	}
	close(tasks)
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	var completedTransfer Transfer
	if err = s.jsonRequest(ctx, http.MethodPost, t.RemoteAddress+"/v1/transfers/"+url.PathEscape(t.ID)+"/complete", t.DeviceID, t.SessionToken, nil, &completedTransfer); err != nil {
		return fmt.Errorf("finalize remote transfer: %w", err)
	}
	latest, err := s.Get(ctx, t.ID)
	if err != nil {
		return err
	}
	latest.Progress = latest.Size
	_, err = s.finish(ctx, latest, StatusCompleted, "", EventCompleted)
	return err
}

func (s *Service) remoteStatus(ctx context.Context, t Transfer) (ResumeMap, error) {
	var result ResumeMap
	err := s.jsonRequest(ctx, http.MethodGet, t.RemoteAddress+"/v1/transfers/"+url.PathEscape(t.ID)+"/status", t.DeviceID, t.SessionToken, nil, &result)
	return result, err
}

func (s *Service) sendChunk(ctx context.Context, t Transfer, file File, index int64) error {
	handle, err := os.Open(file.SourcePath)
	if err != nil {
		return fmt.Errorf("open source %s: %w", file.RelativePath, err)
	}
	defer handle.Close()
	size := file.ChunkSize
	if remaining := file.Size - index*file.ChunkSize; remaining < size {
		size = remaining
	}
	contents := make([]byte, size)
	read, readErr := handle.ReadAt(contents, index*file.ChunkSize)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if int64(read) != size {
		return io.ErrUnexpectedEOF
	}
	checksum := checksumBytes(contents)
	body := contents
	encoding := ""
	if t.Compression && compressible(file.RelativePath) {
		var compressed bytes.Buffer
		writer, _ := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
		_, _ = writer.Write(contents)
		_ = writer.Close()
		if compressed.Len()+128 < len(contents) {
			body = compressed.Bytes()
			encoding = "gzip"
		}
	}
	endpoint := fmt.Sprintf("%s/v1/transfers/%s/files/%s/chunks/%d", t.RemoteAddress, url.PathEscape(t.ID), url.PathEscape(file.ID), index)
	var lastErr error
	for attempt := 1; attempt <= chunkRetryLimit; attempt++ {
		if err := s.waitForThrottle(ctx); err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+t.SessionToken)
		request.Header.Set("Content-Type", "application/octet-stream")
		request.Header.Set("X-Chunk-SHA256", checksum)
		request.Header.Set("X-Chunk-Size", strconv.FormatInt(size, 10))
		if encoding != "" {
			request.Header.Set("Content-Encoding", encoding)
		}
		client, clientErr := s.peerClient(ctx, t.DeviceID)
		if clientErr != nil {
			return clientErr
		}
		response, err := client.Do(request)
		if err == nil {
			io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				s.recordCongestion(false)
				return nil
			}
			err = fmt.Errorf("receiver returned %s", response.Status)
		}
		lastErr = err
		s.recordCongestion(true)
		s.logger.Warn("Transfer chunk retry", "transfer_id", t.ID, "file_id", file.ID, "chunk", index, "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
		}
	}
	return fmt.Errorf("send chunk after %d attempts: %w", chunkRetryLimit, lastErr)
}

func (s *Service) jsonRequest(ctx context.Context, method, endpoint, deviceID, token string, input, output any) error {
	var body io.Reader
	var encoded []byte
	if input != nil {
		var err error
		encoded, err = json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	} else if offer, ok := input.(Offer); ok {
		sharedKey, err := s.authorizer.SharedKey(ctx, deviceID)
		if err != nil {
			return err
		}
		timestamp, nonce := strconv.FormatInt(s.now().UTC().Unix(), 10), uuid.NewString()
		request.Header.Set("X-SyncSpace-Device-ID", offer.DeviceID)
		request.Header.Set("X-SyncSpace-Timestamp", timestamp)
		request.Header.Set("X-SyncSpace-Nonce", nonce)
		request.Header.Set("X-SyncSpace-Signature", offerSignature(sharedKey, method, request.URL.Path, offer.DeviceID, timestamp, nonce, encoded))
	}
	client, err := s.peerClient(ctx, deviceID)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("remote returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	if output == nil {
		_, err = io.Copy(io.Discard, response.Body)
		return err
	}
	return json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(output)
}

func compressible(path string) bool {
	kind := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	return strings.HasPrefix(kind, "text/") || strings.Contains(kind, "json") || strings.Contains(kind, "xml") || strings.Contains(kind, "javascript") || strings.Contains(kind, "svg")
}
