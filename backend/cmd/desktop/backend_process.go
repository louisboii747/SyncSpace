package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type backendProcess struct {
	command *exec.Cmd
	done    chan error
	log     io.Closer
	owner   io.Closer
}

func startBackend(ctx context.Context) (*backendProcess, error) {
	binary, err := backendPath()
	if err != nil {
		return nil, err
	}
	logFile, err := backendLog()
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, binary)
	command.Stdout, command.Stderr = logFile, logFile
	configureBackendProcess(command)
	if err = command.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start secure transfer services: %w", err)
	}
	owner, err := attachBackendProcess(command.Process)
	if err != nil {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
		_ = logFile.Close()
		return nil, fmt.Errorf("secure companion ownership: %w", err)
	}
	process := &backendProcess{command: command, done: make(chan error, 1), log: logFile, owner: owner}
	go func() { process.done <- command.Wait() }()
	if err = waitForBackend(process, 20*time.Second); err != nil {
		process.stop()
		return nil, err
	}
	return process, nil
}

func backendPath() (string, error) {
	if configured := os.Getenv("SYNCSPACE_SERVER_PATH"); configured != "" {
		return configured, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	name := "syncspace-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidates := []string{filepath.Join(filepath.Dir(executable), name)}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, strings.TrimSuffix(executable, ".exe")+"-server.exe")
	}
	if runtime.GOOS == "linux" {
		candidates = append(candidates, "/usr/libexec/syncspace/syncspace-server")
	}
	for _, candidate := range candidates {
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("SyncSpace backend was not found beside the desktop executable")
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
	return os.OpenFile(filepath.Join(directory, "desktop-backend.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

func waitForBackend(process *backendProcess, timeout time.Duration) error {
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-process.done:
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

func (p *backendProcess) stop() {
	if p == nil || p.command == nil || p.command.Process == nil {
		return
	}
	_ = interruptBackend(p.command.Process)
	select {
	case <-p.done:
	case <-time.After(8 * time.Second):
		_ = p.command.Process.Kill()
		<-p.done
	}
	if p.log != nil {
		_ = p.log.Close()
	}
	if p.owner != nil {
		_ = p.owner.Close()
	}
}
