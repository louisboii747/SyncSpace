package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const (
	DefaultChunkSize int64 = 4 << 20
	MaximumChunkSize int64 = 16 << 20
)

func buildManifest(paths []string, chunkSize int64) ([]File, int64, string, error) {
	if len(paths) == 0 {
		return nil, 0, "", fmt.Errorf("%w: at least one path is required", ErrInvalidRequest)
	}
	if chunkSize <= 0 || chunkSize > MaximumChunkSize {
		return nil, 0, "", fmt.Errorf("%w: invalid chunk size", ErrInvalidRequest)
	}
	files := make([]File, 0)
	seen := make(map[string]struct{})
	for _, raw := range paths {
		absolute, err := filepath.Abs(strings.TrimSpace(raw))
		if err != nil {
			return nil, 0, "", err
		}
		info, err := os.Lstat(absolute)
		if err != nil {
			return nil, 0, "", fmt.Errorf("inspect %q: %w", raw, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, 0, "", fmt.Errorf("%w: symbolic links are not transferred", ErrInvalidRequest)
		}
		rootParent := filepath.Dir(absolute)
		err = filepath.WalkDir(absolute, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("%w: symbolic links are not transferred", ErrInvalidRequest)
			}
			relative, err := filepath.Rel(rootParent, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if err := validateRelativePath(relative); err != nil {
				return err
			}
			key := strings.ToLower(relative)
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%w: duplicate relative path %q", ErrInvalidRequest, relative)
			}
			seen[key] = struct{}{}
			item := File{ID: uuid.NewString(), RelativePath: relative, SourcePath: path, Directory: entry.IsDir(), ChunkSize: chunkSize}
			if entry.IsDir() {
				files = append(files, item)
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("%w: %q is not a regular file", ErrInvalidRequest, path)
			}
			item.Size = info.Size()
			item.ChunkCount = chunkCount(item.Size, chunkSize)
			item.Checksum, err = checksumFile(path)
			if err != nil {
				return fmt.Errorf("checksum %q: %w", path, err)
			}
			files = append(files, item)
			return nil
		})
		if err != nil {
			return nil, 0, "", err
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelativePath < files[j].RelativePath })
	aggregate := sha256.New()
	var total int64
	for _, file := range files {
		if file.Directory {
			continue
		}
		total += file.Size
		_, _ = io.WriteString(aggregate, file.RelativePath)
		_, _ = io.WriteString(aggregate, "\x00"+file.Checksum+"\x00")
	}
	return files, total, hex.EncodeToString(aggregate.Sum(nil)), nil
}

func checksumFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	buffer := make([]byte, 1<<20)
	if _, err = io.CopyBuffer(hash, file, buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func checksumBytes(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func chunkCount(size, chunkSize int64) int64 {
	if size == 0 {
		return 0
	}
	return (size + chunkSize - 1) / chunkSize
}

func validateRelativePath(value string) error {
	if value == "" || strings.ContainsRune(value, '\x00') || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return ErrPathTraversal
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != value || filepath.IsAbs(value) {
		return ErrPathTraversal
	}
	if volume := filepath.VolumeName(value); volume != "" {
		return ErrPathTraversal
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." || isWindowsReservedName(part) {
			return ErrPathTraversal
		}
	}
	return nil
}

func secureJoin(root, relative string) (string, error) {
	if err := validateRelativePath(relative); err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(rootAbs, filepath.FromSlash(relative))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathTraversal
	}
	return targetAbs, nil
}

func isWindowsReservedName(name string) bool {
	base := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	}
	return strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ")
}

func resolveConflict(path string, policy ConflictPolicy) (string, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	switch policy {
	case ConflictOverwrite:
		return path, nil
	case ConflictRename:
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(filepath.Base(path), ext)
		dir := filepath.Dir(path)
		for i := 1; i < 10000; i++ {
			candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
			if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
				return candidate, nil
			} else if err != nil {
				return "", err
			}
		}
		return "", errors.New("unable to allocate a non-conflicting filename")
	default:
		return "", ErrConflictResolution
	}
}
