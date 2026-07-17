package services

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestFileIdentityStorePersistsIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "identity.json")
	store := NewFileIdentityStore(path)
	first, err := store.LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.PublicKey != second.PublicKey || first.Fingerprint != second.Fingerprint || !bytes.Equal(first.PrivateKey, second.PrivateKey) {
		t.Fatalf("identity changed between loads: %#v != %#v", first, second)
	}
	if _, err := uuid.Parse(first.ID); err != nil {
		t.Fatalf("identity did not contain a UUID: %v", err)
	}
	metadata, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(metadata, first.PrivateKey) {
		t.Fatal("identity metadata leaked the private key")
	}
	var encoded map[string]any
	if err := json.Unmarshal(metadata, &encoded); err != nil {
		t.Fatal(err)
	}
	if _, exists := encoded["privateKey"]; exists {
		t.Fatal("private key field was serialized")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "identity.key")); err != nil {
		t.Fatalf("protected key file missing: %v", err)
	}
}

func TestFileIdentityStoreMigratesLegacyMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	legacy := `{"deviceId":"00000000-0000-4000-8000-000000000001","deviceName":"Legacy","deviceType":"desktop","platform":"windows"}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := NewFileIdentityStore(path).LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	if identity.PublicKey == "" || identity.Fingerprint == "" || len(identity.PrivateKey) == 0 {
		t.Fatal("legacy identity was not upgraded")
	}
}

func TestFileIdentityStoreDoesNotReplaceCorruptIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewFileIdentityStore(path)
	if _, err := store.LoadOrCreate(); err == nil {
		t.Fatal("expected corrupt identity to fail instead of being regenerated")
	}
}
