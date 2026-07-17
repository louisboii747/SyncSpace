// Package services contains application services that are independent from
// HTTP, WebSocket, and mDNS transports.
package services

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const identityProtocolVersion = 1

// Identity is the permanent cryptographic identity of one SyncSpace
// installation. PrivateKey is populated in memory only and is never encoded in
// API responses, discovery records, logs, or identity.json.
type Identity struct {
	ID              string             `json:"deviceId"`
	Name            string             `json:"deviceName"`
	Type            string             `json:"deviceType"`
	Platform        string             `json:"platform"`
	ProtocolVersion int                `json:"protocolVersion"`
	PublicKey       string             `json:"publicKey"`
	Fingerprint     string             `json:"fingerprint"`
	CreatedAt       time.Time          `json:"createdAt"`
	ModifiedAt      time.Time          `json:"modifiedAt"`
	PrivateKey      ed25519.PrivateKey `json:"-"`
}

// IdentityStore loads or creates a permanent device identity.
type IdentityStore interface {
	LoadOrCreate() (Identity, error)
}

// FileIdentityStore stores public metadata atomically in identity.json and the
// private Ed25519 key separately. Windows protects the key with user-scoped
// DPAPI; other platforms use a mode-0600 fallback that platform shells can
// replace with Keychain, Keystore, or Secret Service adapters.
type FileIdentityStore struct {
	path string
	mu   sync.Mutex
}

// NewFileIdentityStore creates an identity store at path.
func NewFileIdentityStore(path string) *FileIdentityStore { return &FileIdentityStore{path: path} }

// LoadOrCreate returns the existing identity, migrates legacy metadata by
// generating a cryptographic key, or atomically persists a new identity.
func (s *FileIdentityStore) LoadOrCreate() (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	identity, err := s.loadMetadata()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Identity{}, err
	}
	if errors.Is(err, os.ErrNotExist) {
		identity = newIdentityMetadata()
	}

	privateKey, keyErr := s.loadPrivateKey()
	if keyErr != nil && !errors.Is(keyErr, os.ErrNotExist) {
		return Identity{}, keyErr
	}
	metadataChanged := false
	if errors.Is(keyErr, os.ErrNotExist) {
		publicKey, generated, generateErr := ed25519.GenerateKey(rand.Reader)
		if generateErr != nil {
			return Identity{}, fmt.Errorf("generate identity key: %w", generateErr)
		}
		privateKey = generated
		identity.PublicKey = base64.RawURLEncoding.EncodeToString(publicKey)
		identity.Fingerprint = IdentityFingerprint(publicKey)
		metadataChanged = true
		if err := s.savePrivateKey(privateKey); err != nil {
			return Identity{}, err
		}
	}

	publicKey := privateKey.Public().(ed25519.PublicKey)
	encodedPublicKey := base64.RawURLEncoding.EncodeToString(publicKey)
	if identity.PublicKey == "" {
		identity.PublicKey = encodedPublicKey
		identity.Fingerprint = IdentityFingerprint(publicKey)
		metadataChanged = true
	} else if identity.PublicKey != encodedPublicKey || identity.Fingerprint != IdentityFingerprint(publicKey) {
		return Identity{}, errors.New("identity metadata does not match the protected private key")
	}
	if identity.ProtocolVersion == 0 {
		identity.ProtocolVersion = identityProtocolVersion
		metadataChanged = true
	}
	now := time.Now().UTC()
	if identity.CreatedAt.IsZero() {
		identity.CreatedAt = now
		metadataChanged = true
	}
	if identity.ModifiedAt.IsZero() {
		identity.ModifiedAt = identity.CreatedAt
		metadataChanged = true
	}
	identity.PrivateKey = privateKey
	if err := identity.Validate(); err != nil {
		return Identity{}, err
	}
	if metadataChanged || errors.Is(err, os.ErrNotExist) {
		if err := s.saveMetadata(identity); err != nil {
			return Identity{}, err
		}
	}
	return identity, nil
}

func (s *FileIdentityStore) keyPath() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + ".key"
}

func (s *FileIdentityStore) loadMetadata() (Identity, error) {
	contents, err := os.ReadFile(s.path)
	if err != nil {
		return Identity{}, err
	}
	var identity Identity
	if err := json.Unmarshal(contents, &identity); err != nil {
		return Identity{}, fmt.Errorf("decode identity: %w", err)
	}
	return identity, nil
}

func (s *FileIdentityStore) saveMetadata(identity Identity) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create identity directory: %w", err)
	}
	contents, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return fmt.Errorf("encode identity: %w", err)
	}
	return atomicWritePrivateFile(s.path, append(contents, '\n'))
}

func (s *FileIdentityStore) loadPrivateKey() (ed25519.PrivateKey, error) {
	protected, err := os.ReadFile(s.keyPath())
	if err != nil {
		return nil, err
	}
	contents, err := unprotectIdentitySecret(protected)
	if err != nil {
		return nil, fmt.Errorf("unprotect identity key: %w", err)
	}
	if len(contents) != ed25519.PrivateKeySize {
		return nil, errors.New("protected identity key has an invalid length")
	}
	return ed25519.PrivateKey(append([]byte(nil), contents...)), nil
}

func (s *FileIdentityStore) savePrivateKey(privateKey ed25519.PrivateKey) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create identity directory: %w", err)
	}
	protected, err := protectIdentitySecret(privateKey)
	if err != nil {
		return fmt.Errorf("protect identity key: %w", err)
	}
	if err := atomicWritePrivateFile(s.keyPath(), protected); err != nil {
		return fmt.Errorf("save protected identity key: %w", err)
	}
	return nil
}

func atomicWritePrivateFile(path string, contents []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".syncspace-identity-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// Validate checks that a persisted identity is safe to use and advertise.
func (i Identity) Validate() error {
	if _, err := uuid.Parse(i.ID); err != nil {
		return fmt.Errorf("invalid device ID: %w", err)
	}
	if strings.TrimSpace(i.Name) == "" || len(i.Name) > 128 {
		return errors.New("device name must contain 1 to 128 characters")
	}
	if strings.TrimSpace(i.Type) == "" || len(i.Type) > 32 || strings.TrimSpace(i.Platform) == "" || len(i.Platform) > 32 {
		return errors.New("device type and platform are required")
	}
	if i.ProtocolVersion != identityProtocolVersion || len(i.PrivateKey) != ed25519.PrivateKeySize {
		return errors.New("identity protocol version or private key is invalid")
	}
	publicKey, err := base64.RawURLEncoding.DecodeString(i.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || i.Fingerprint != IdentityFingerprint(publicKey) {
		return errors.New("identity public key or fingerprint is invalid")
	}
	if i.CreatedAt.IsZero() || i.ModifiedAt.IsZero() {
		return errors.New("identity timestamps are required")
	}
	return nil
}

// IdentityFingerprint returns the full SHA-256 fingerprint used for visual
// comparison and pinned trust records.
func IdentityFingerprint(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	raw := strings.ToUpper(hex.EncodeToString(sum[:]))
	parts := make([]string, 0, len(raw)/4)
	for index := 0; index < len(raw); index += 4 {
		parts = append(parts, raw[index:index+4])
	}
	return strings.Join(parts, ":")
}

// ShortFingerprint is safe for compact discovery records. Pairing always shows
// the complete fingerprint and a separately derived verification code.
func (i Identity) ShortFingerprint() string {
	parts := strings.Split(i.Fingerprint, ":")
	if len(parts) > 4 {
		parts = parts[:4]
	}
	return strings.Join(parts, ":")
}

func newIdentityMetadata() Identity {
	now := time.Now().UTC()
	return Identity{ID: uuid.NewString(), Name: friendlyDeviceName(), Type: currentDeviceType(), Platform: runtime.GOOS, ProtocolVersion: identityProtocolVersion, CreatedAt: now, ModifiedAt: now}
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
