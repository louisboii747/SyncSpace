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
	"sync"
)

const SchemaVersion = 1

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
}

type Store struct {
	path   string
	mu     sync.RWMutex
	values Values
}

func NewStore(path string) (*Store, error) {
	store := &Store{path: path}
	values, err := store.load()
	if errors.Is(err, os.ErrNotExist) {
		values = Defaults()
		if err := store.save(values); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	store.values = values
	return store, nil
}

func Defaults() Values {
	destination := ""
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, "Downloads")
		if filepath.IsAbs(candidate) {
			destination = candidate
		}
	}
	return Values{SchemaVersion: SchemaVersion, Appearance: AppearanceSystem, DefaultDownloadDirectory: destination, ConflictPolicy: "rename", NotificationsEnabled: true}
}

func (s *Store) Get() Values { s.mu.RLock(); defer s.mu.RUnlock(); return s.values }

func (s *Store) Update(values Values) (Values, error) {
	if err := Validate(values); err != nil {
		return Values{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	values.SchemaVersion = SchemaVersion
	if err := s.save(values); err != nil {
		return Values{}, err
	}
	s.values = values
	return values, nil
}

func Validate(values Values) error {
	if values.SchemaVersion != 0 && values.SchemaVersion != SchemaVersion {
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
	values.SchemaVersion = SchemaVersion
	return values, nil
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
