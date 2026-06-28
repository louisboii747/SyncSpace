package services

import (
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
	if first != second {
		t.Fatalf("identity changed between loads: %#v != %#v", first, second)
	}
	if _, err := uuid.Parse(first.ID); err != nil {
		t.Fatalf("identity did not contain a UUID: %v", err)
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
