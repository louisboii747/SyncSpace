package settings

import (
	"path/filepath"
	"testing"
)

func TestStorePersistsValidatedSettingsAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	values := store.Get()
	values.Appearance = AppearanceDark
	values.ReducedMotion = true
	values.ConflictPolicy = "prompt"
	if _, err := store.Update(values); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Get().Appearance != AppearanceDark || !reloaded.Get().ReducedMotion || reloaded.Get().ConflictPolicy != "prompt" {
		t.Fatalf("settings did not persist: %#v", reloaded.Get())
	}
}

func TestStoreRejectsRelativeDownloadDirectory(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	values := store.Get()
	values.DefaultDownloadDirectory = "relative/path"
	if _, err := store.Update(values); err == nil {
		t.Fatal("relative destination was accepted")
	}
}
