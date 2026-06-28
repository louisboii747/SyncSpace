// Package services contains application services that are independent from
// HTTP, WebSocket, and mDNS transports.
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// Identity is the permanent, locally stored identity of this SyncSpace
// installation. Runtime attributes such as IP, port, and app version are not
// persisted here.
type Identity struct {
	ID       string `json:"deviceId"`
	Name     string `json:"deviceName"`
	Type     string `json:"deviceType"`
	Platform string `json:"platform"`
}

// IdentityStore loads or creates a permanent device identity.
type IdentityStore interface {
	LoadOrCreate() (Identity, error)
}

// FileIdentityStore persists identity as a small atomic JSON file.
type FileIdentityStore struct {
	path string
	mu   sync.Mutex
}

// NewFileIdentityStore creates an identity store at path.
func NewFileIdentityStore(path string) *FileIdentityStore {
	return &FileIdentityStore{path: path}
}

// LoadOrCreate returns the existing identity, or atomically persists a new one
// on first launch.
func (s *FileIdentityStore) LoadOrCreate() (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, err := s.load()
	if err == nil {
		return identity, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Identity{}, err
	}

	identity = newIdentity()
	if err := identity.Validate(); err != nil {
		return Identity{}, err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return Identity{}, fmt.Errorf("create identity directory: %w", err)
	}

	contents, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return Identity{}, fmt.Errorf("encode identity: %w", err)
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, append(contents, '\n'), 0o600); err != nil {
		return Identity{}, fmt.Errorf("write identity: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		_ = os.Remove(temporary)
		return Identity{}, fmt.Errorf("commit identity: %w", err)
	}
	return identity, nil
}

func (s *FileIdentityStore) load() (Identity, error) {
	contents, err := os.ReadFile(s.path)
	if err != nil {
		return Identity{}, err
	}
	var identity Identity
	if err := json.Unmarshal(contents, &identity); err != nil {
		return Identity{}, fmt.Errorf("decode identity: %w", err)
	}
	if err := identity.Validate(); err != nil {
		return Identity{}, fmt.Errorf("validate identity: %w", err)
	}
	return identity, nil
}

// Validate checks that a persisted identity is safe to advertise.
func (i Identity) Validate() error {
	if _, err := uuid.Parse(i.ID); err != nil {
		return fmt.Errorf("invalid device ID: %w", err)
	}
	if strings.TrimSpace(i.Name) == "" || len(i.Name) > 128 {
		return errors.New("device name must contain 1 to 128 characters")
	}
	if strings.TrimSpace(i.Type) == "" || len(i.Type) > 32 ||
		strings.TrimSpace(i.Platform) == "" || len(i.Platform) > 32 {
		return errors.New("device type and platform are required")
	}
	return nil
}

func newIdentity() Identity {
	return Identity{
		ID:       uuid.NewString(),
		Name:     friendlyDeviceName(),
		Type:     currentDeviceType(),
		Platform: runtime.GOOS,
	}
}

func friendlyDeviceName() string {
	hostname, err := os.Hostname()
	if err == nil {
		hostname = strings.TrimSpace(strings.TrimSuffix(hostname, ".local"))
		if hostname != "" {
			if len(hostname) > 128 {
				return hostname[:128]
			}
			return hostname
		}
	}
	switch runtime.GOOS {
	case "darwin":
		return "Mac"
	case "ios":
		return "iPhone"
	case "android":
		return "Android Device"
	case "windows":
		return "Windows PC"
	default:
		return "SyncSpace Device"
	}
}

func currentDeviceType() string {
	switch runtime.GOOS {
	case "android", "ios":
		return "mobile"
	default:
		return "desktop"
	}
}
