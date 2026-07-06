package transfer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildManifestSupportsFoldersMultipleSelectionsAndEmptyDirectories(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "photos")
	if err := os.MkdirAll(filepath.Join(folder, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "one.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "notes.md")
	if err := os.WriteFile(other, []byte("world"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, size, aggregate, err := buildManifest([]string{folder, other}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if size != 10 || len(aggregate) != 64 {
		t.Fatalf("unexpected aggregate: size=%d checksum=%q", size, aggregate)
	}
	paths := map[string]File{}
	for _, file := range files {
		paths[file.RelativePath] = file
	}
	if !paths["photos/empty"].Directory {
		t.Fatalf("empty directory missing from manifest: %#v", files)
	}
	if paths["photos/one.txt"].ChunkCount != 2 || paths["notes.md"].ChunkCount != 2 {
		t.Fatalf("unexpected chunk counts: %#v", files)
	}
}

func TestPathValidationRejectsTraversalAndPortableReservedNames(t *testing.T) {
	invalid := []string{"../secret", "folder/../../secret", "/absolute", `folder\secret`, "folder/CON.txt", "folder/trailing. "}
	for _, value := range invalid {
		if err := validateRelativePath(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
	root := t.TempDir()
	joined, err := secureJoin(root, "safe/nested.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(joined, root) {
		t.Fatalf("joined path escaped root: %q", joined)
	}
}

func TestChunkCountHandlesHundredGigabyteFiles(t *testing.T) {
	const size = int64(100) * 1024 * 1024 * 1024
	if got, want := chunkCount(size, DefaultChunkSize), int64(25600); got != want {
		t.Fatalf("chunkCount=%d want %d", got, want)
	}
}
