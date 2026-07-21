package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestPrivacyAcceptanceIsVersionedAndCannotBeForgedByPreferenceUpdate(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if store.PrivacyAccepted() {
		t.Fatal("new store unexpectedly accepted the privacy policy")
	}
	if _, err := store.AcceptPrivacy("old-version", time.Now()); err == nil {
		t.Fatal("stale privacy policy version was accepted")
	}
	acceptedAt := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	accepted, err := store.AcceptPrivacy(CurrentPrivacyPolicyVersion, acceptedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !store.PrivacyAccepted() || accepted.PrivacyAcceptedAt == nil || !accepted.PrivacyAcceptedAt.Equal(acceptedAt) {
		t.Fatalf("privacy acceptance was not recorded: %#v", accepted)
	}
	accepted.PrivacyPolicyVersion = "forged"
	accepted.PrivacyAcceptedAt = nil
	accepted.Appearance = AppearanceDark
	if _, err := store.Update(accepted); err == nil {
		t.Fatal("forged acceptance fields were accepted")
	}
	accepted = store.Get()
	accepted.Appearance = AppearanceDark
	updated, err := store.Update(accepted)
	if err != nil {
		t.Fatal(err)
	}
	if !store.PrivacyAccepted() || updated.PrivacyPolicyVersion != CurrentPrivacyPolicyVersion || updated.PrivacyAcceptedAt == nil {
		t.Fatalf("preference update changed policy acceptance: %#v", updated)
	}
}

func TestStoreMigratesSchemaOneWithoutAssumingPrivacyAcceptance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := `{"schemaVersion":1,"appearance":"dark","reducedMotion":false,"defaultDownloadDirectory":"","conflictPolicy":"rename","notificationsEnabled":true}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	values := store.Get()
	if values.SchemaVersion != SchemaVersion || !values.Discoverable || !values.IncomingTransfersEnabled || store.PrivacyAccepted() {
		t.Fatalf("unexpected migrated values: %#v", values)
	}
}

func TestDefaultReceiveDirectoryUsesSyncSpaceSubfolder(t *testing.T) {
	if got := filepath.Clean(Defaults().DefaultDownloadDirectory); !strings.HasSuffix(got, filepath.Join("Downloads", "SyncSpace")) {
		t.Fatalf("default receive directory = %q", got)
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
