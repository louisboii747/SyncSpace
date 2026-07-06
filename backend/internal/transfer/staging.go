package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const stagingBufferSize = 1 << 20

// CreateStage creates an isolated upload area for browser-selected files.
// Staging is intentionally separate from durable transfer state: only a fully
// uploaded session can be promoted into Queue.
func (s *Service) CreateStage(_ context.Context) (StagingSession, error) {
	now := s.now().UTC()
	session := StagingSession{ID: uuid.NewString(), CreatedAt: now}
	root := s.uploadingStagePath(session.ID)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return StagingSession{}, fmt.Errorf("create transfer staging session: %w", err)
	}
	if err := os.Chtimes(root, now, now); err != nil {
		return StagingSession{}, fmt.Errorf("timestamp transfer staging session: %w", err)
	}
	return session, nil
}

// UploadStagedFile streams one browser-selected file directly to disk. It
// never buffers the whole body and refuses duplicate paths within a session.
func (s *Service) UploadStagedFile(ctx context.Context, sessionID, relativePath string, size int64, contents io.Reader) error {
	if _, err := uuid.Parse(sessionID); err != nil {
		return fmt.Errorf("%w: invalid staging session", ErrInvalidRequest)
	}
	if err := validateRelativePath(relativePath); err != nil {
		return err
	}
	if size < 0 {
		return fmt.Errorf("%w: content length is required", ErrInvalidRequest)
	}
	root := s.uploadingStagePath(sessionID)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return ErrInvalidRequest
	}
	s.beginStageUpload(sessionID)
	defer s.endStageUpload(sessionID)
	if available := availableStorage(root); available >= 0 && size > available {
		return fmt.Errorf("%w: insufficient storage", ErrInvalidRequest)
	}
	target, err := secureJoin(root, relativePath)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	handle, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w: staged path already exists", ErrInvalidRequest)
		}
		return err
	}
	completed := false
	defer func() {
		_ = handle.Close()
		if !completed {
			_ = os.Remove(target)
		}
	}()

	written, copyErr := io.CopyBuffer(handle, io.LimitReader(&contextReader{ctx: ctx, reader: contents}, size+1), make([]byte, stagingBufferSize))
	if copyErr != nil {
		return copyErr
	}
	if written != size {
		return fmt.Errorf("%w: uploaded size does not match content length", ErrInvalidRequest)
	}
	if err = handle.Sync(); err != nil {
		return err
	}
	if err = handle.Close(); err != nil {
		return err
	}
	completed = true
	return nil
}

// QueueStage seals an upload session and promotes its selected roots into the
// same persistent queue used by native clients.
func (s *Service) QueueStage(ctx context.Context, sessionID string, request StageQueueRequest) (Transfer, error) {
	if _, err := uuid.Parse(sessionID); err != nil {
		return Transfer{}, fmt.Errorf("%w: invalid staging session", ErrInvalidRequest)
	}
	if len(request.Roots) == 0 {
		return Transfer{}, fmt.Errorf("%w: staged roots are required", ErrInvalidRequest)
	}
	if s.stageUploadActive(sessionID) {
		return Transfer{}, fmt.Errorf("%w: staging upload is still active", ErrInvalidState)
	}
	uploading := s.uploadingStagePath(sessionID)
	queued := s.queuedStagePath(sessionID)
	paths := make([]string, 0, len(request.Roots))
	seen := make(map[string]struct{}, len(request.Roots))
	for _, rootName := range request.Roots {
		rootName = strings.TrimSpace(filepath.ToSlash(rootName))
		if err := validateRelativePath(rootName); err != nil || strings.Contains(rootName, "/") {
			return Transfer{}, fmt.Errorf("%w: roots must be top-level staged names", ErrInvalidRequest)
		}
		key := strings.ToLower(rootName)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		candidate, err := secureJoin(uploading, rootName)
		if err != nil {
			return Transfer{}, err
		}
		if _, err = os.Lstat(candidate); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return Transfer{}, fmt.Errorf("%w: staged root %q does not exist", ErrInvalidRequest, rootName)
			}
			return Transfer{}, err
		}
		paths = append(paths, filepath.Join(queued, filepath.FromSlash(rootName)))
	}
	if err := os.MkdirAll(filepath.Dir(queued), 0o700); err != nil {
		return Transfer{}, err
	}
	if err := os.Rename(uploading, queued); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Transfer{}, ErrNotFound
		}
		return Transfer{}, err
	}
	result, err := s.Queue(ctx, QueueRequest{DeviceID: request.DeviceID, Paths: paths, ConflictPolicy: request.ConflictPolicy})
	if err != nil {
		if rollbackErr := os.Rename(queued, uploading); rollbackErr != nil {
			s.logger.Error("Unable to roll back transfer staging session", "session_id", sessionID, "error", rollbackErr)
		}
		return Transfer{}, err
	}
	return result, nil
}

func (s *Service) DeleteStage(_ context.Context, sessionID string) error {
	if _, err := uuid.Parse(sessionID); err != nil {
		return fmt.Errorf("%w: invalid staging session", ErrInvalidRequest)
	}
	if s.stageUploadActive(sessionID) {
		return fmt.Errorf("%w: staging upload is still active", ErrInvalidState)
	}
	if err := os.RemoveAll(s.uploadingStagePath(sessionID)); err != nil {
		return fmt.Errorf("remove transfer staging session: %w", err)
	}
	return nil
}

func (s *Service) uploadingStagePath(id string) string {
	return filepath.Join(s.dataDirectory, "staging", "uploading", id)
}

func (s *Service) queuedStagePath(id string) string {
	return filepath.Join(s.dataDirectory, "staging", "queued", id)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(buffer)
	}
}

func (s *Service) removeExpiredStages(now time.Time) {
	root := filepath.Join(s.dataDirectory, "staging", "uploading")
	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logger.Warn("Unable to inspect transfer staging sessions", "error", err)
		return
	}
	for _, entry := range entries {
		if s.stageUploadActive(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr == nil && info.IsDir() && now.Sub(info.ModTime()) > 24*time.Hour {
			if removeErr := os.RemoveAll(filepath.Join(root, entry.Name())); removeErr != nil {
				s.logger.Warn("Unable to expire transfer staging session", "session_id", entry.Name(), "error", removeErr)
			}
		}
	}
}

func (s *Service) beginStageUpload(id string) {
	s.stagingMu.Lock()
	s.stagingActive[id]++
	s.stagingMu.Unlock()
}

func (s *Service) endStageUpload(id string) {
	s.stagingMu.Lock()
	if s.stagingActive[id] <= 1 {
		delete(s.stagingActive, id)
	} else {
		s.stagingActive[id]--
	}
	s.stagingMu.Unlock()
}

func (s *Service) stageUploadActive(id string) bool {
	s.stagingMu.Lock()
	defer s.stagingMu.Unlock()
	return s.stagingActive[id] > 0
}
