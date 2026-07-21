// Package settings owns the small, atomically persisted set of user-facing
// preferences that are currently honored by both the local UI and transfer
// approval workflow.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	SchemaVersion               = 2
	CurrentPrivacyPolicyVersion = "2026-07-1"
)

type Appearance string

const (
	AppearanceSystem Appearance = "system"
	AppearanceLight  Appearance = "light"
	AppearanceDark   Appearance = "dark"
)

type Values struct {
	SchemaVersion            int        `json:"schemaVersion"`
	Appearance               Appearance `json:"appearance"`
	ReducedMotion            bool       `json:"reducedMotion"`
	DefaultDownloadDirectory string     `json:"defaultDownloadDirectory"`
	ConflictPolicy           string     `json:"conflictPolicy"`
	NotificationsEnabled     bool       `json:"notificationsEnabled"`
	DeviceName               string     `json:"deviceName"`
	Discoverable             bool       `json:"discoverable"`
	IncomingTransfersEnabled bool       `json:"incomingTransfersEnabled"`
	PrivacyPolicyVersion     string     `json:"privacyPolicyVersion"`
	PrivacyAcceptedAt        *time.Time `json:"privacyAcceptedAt,omitempty"`
}

type Store struct {
	path    string
	mu      sync.RWMutex
	values  Values
	changed chan struct{}
}

func NewStore(path string) (*Store, error) {
	store := &Store{path: path, changed: make(chan struct{}, 1)}
	values, err := store.load()
	if errors.Is(err, os.ErrNotExist) {
		values = Defaults()
	} else if err != nil {
		return nil, err
	}
	values = migrate(values)
	if err := store.save(values); err != nil {
		return nil, err
	}
	store.values = values
	return store, nil
}

func Defaults() Values {
	destination := ""
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, "Downloads", "SyncSpace")
		if filepath.IsAbs(candidate) {
			destination = candidate
		}
	}
	return Values{
		SchemaVersion: SchemaVersion, Appearance: AppearanceSystem,
		DefaultDownloadDirectory: destination, ConflictPolicy: "rename",
		NotificationsEnabled: false, Discoverable: true, IncomingTransfersEnabled: true,
	}
}

func (s *Store) Get() Values { s.mu.RLock(); defer s.mu.RUnlock(); return s.values }

// Changes is a coalesced signal used by runtime services that must react to
// privacy, discoverability, or display-name changes without polling files.
func (s *Store) Changes() <-chan struct{} { return s.changed }

// PrivacyAccepted reports whether the user accepted the policy currently
// shipped with this build. A previous policy version is deliberately stale.
func (s *Store) PrivacyAccepted() bool {
	values := s.Get()
	return values.PrivacyAcceptedAt != nil && values.PrivacyPolicyVersion == CurrentPrivacyPolicyVersion
}

func (s *Store) Update(values Values) (Values, error) {
	values.DeviceName = strings.TrimSpace(values.DeviceName)
	if err := Validate(values); err != nil {
		return Values{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// General preference writes cannot forge or clear policy acceptance. That
	// state is changed only by the explicit, version-checked acceptance route.
	values.PrivacyPolicyVersion = s.values.PrivacyPolicyVersion
	values.PrivacyAcceptedAt = s.values.PrivacyAcceptedAt
	values.SchemaVersion = SchemaVersion
	if err := s.save(values); err != nil {
		return Values{}, err
	}
	s.values = values
	s.notify()
	return values, nil
}

// AcceptPrivacy records an explicit acceptance of the exact current version.
func (s *Store) AcceptPrivacy(version string, acceptedAt time.Time) (Values, error) {
	if version != CurrentPrivacyPolicyVersion {
		return Values{}, errors.New("privacy policy version is no longer current")
	}
	if acceptedAt.IsZero() {
		acceptedAt = time.Now().UTC()
	}
	acceptedAt = acceptedAt.UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	values := s.values
	values.SchemaVersion = SchemaVersion
	values.PrivacyPolicyVersion = version
	values.PrivacyAcceptedAt = &acceptedAt
	if err := s.save(values); err != nil {
		return Values{}, err
	}
	s.values = values
	s.notify()
	return values, nil
}

func Validate(values Values) error {
	if values.SchemaVersion != 0 && values.SchemaVersion != 1 && values.SchemaVersion != SchemaVersion {
		return errors.New("unsupported settings schema version")
	}
	if values.Appearance != AppearanceSystem && values.Appearance != AppearanceLight && values.Appearance != AppearanceDark {
		return errors.New("appearance must be system, light, or dark")
	}
	if values.ConflictPolicy != "prompt" && values.ConflictPolicy != "rename" && values.ConflictPolicy != "overwrite" {
		return errors.New("conflict policy must be prompt, rename, or overwrite")
	}
	if values.DefaultDownloadDirectory != "" && !filepath.IsAbs(values.DefaultDownloadDirectory) {
		return errors.New("default download directory must be an absolute path")
	}
	if len(values.DefaultDownloadDirectory) > 4096 {
		return errors.New("default download directory is too long")
	}
	if len(values.DeviceName) > 128 || (values.DeviceName != "" && strings.TrimSpace(values.DeviceName) == "") {
		return errors.New("device name must contain 1 to 128 characters")
	}
	if values.PrivacyPolicyVersion != "" && values.PrivacyPolicyVersion != CurrentPrivacyPolicyVersion {
		// Previous versions may exist on disk and are valid persisted state. They
		// simply do not satisfy PrivacyAccepted until the current policy is shown.
		if values.PrivacyAcceptedAt == nil {
			return errors.New("privacy policy version requires an acceptance timestamp")
		}
	}
	return nil
}

func (s *Store) load() (Values, error) {
	contents, err := os.ReadFile(s.path)
	if err != nil {
		return Values{}, err
	}
	var values Values
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&values); err != nil {
		return Values{}, fmt.Errorf("decode settings: %w", err)
	}
	if err := Validate(values); err != nil {
		return Values{}, fmt.Errorf("validate settings: %w", err)
	}
	return values, nil
}

func migrate(values Values) Values {
	if values.SchemaVersion < 2 {
		// Schema v1 predated the privacy gate. Keep its established user
		// preferences but choose safe, usable defaults for the new controls. The
		// policy remains unaccepted and must be shown before network activity.
		values.Discoverable = true
		values.IncomingTransfersEnabled = true
		// Previous releases enabled filename-bearing system notifications by
		// default. Schema v2 makes them an explicit opt-in.
		values.NotificationsEnabled = false
	}
	values.DeviceName = strings.TrimSpace(values.DeviceName)
	values.SchemaVersion = SchemaVersion
	return values
}

func (s *Store) notify() {
	select {
	case s.changed <- struct{}{}:
	default:
	}
}

func (s *Store) save(values Values) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".syncspace-settings-*")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer os.Remove(path)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(contents, '\n')); err != nil {
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
	if err := os.Rename(path, s.path); err != nil {
		return err
	}
	return os.Chmod(s.path, 0o600)
}
