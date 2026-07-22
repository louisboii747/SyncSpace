package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/louisboii747/syncspace/backend/internal/diagnostics"
	"github.com/louisboii747/syncspace/backend/internal/serverapp"
)

type backendRuntime struct {
	cancel context.CancelFunc
	done   chan error
	log    *os.File
}

func startBackend(parent context.Context) (*backendRuntime, error) {
	logFile, err := backendLog()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	logBuffer := diagnostics.NewLogBuffer(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo}), 500)
	logger := slog.New(logBuffer)
	runtime := &backendRuntime{cancel: cancel, done: make(chan error, 1), log: logFile}
	go func() { runtime.done <- serverapp.Run(ctx, logger, logBuffer, buildVersion) }()
	if err = waitForBackend(runtime, 20*time.Second); err != nil {
		cancel()
		_ = logFile.Close()
		return nil, err
	}
	return runtime, nil
}

func backendLog() (*os.File, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(cache, "SyncSpace", "logs")
	if err = os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(directory, "desktop-runtime.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

func waitForBackend(runtime *backendRuntime, timeout time.Duration) error {
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-runtime.done:
			if err == nil {
				return errors.New("secure transfer services stopped during startup")
			}
			return fmt.Errorf("secure transfer services stopped during startup: %w", err)
		default:
		}
		response, err := client.Get("http://127.0.0.1:8384/api/v1/privacy-policy")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("secure transfer services did not become ready")
}

func (r *backendRuntime) stop() {
	if r == nil {
		return
	}
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(12 * time.Second):
	}
	if r.log != nil {
		_ = r.log.Close()
	}
}
