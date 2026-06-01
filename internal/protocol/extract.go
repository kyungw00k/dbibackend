package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// extractEmbeddedLibusb extracts the embedded libusb shared library to a
// per-user cache directory and returns the path. The file is content-addressed
// by a truncated SHA-256 hash, so it is extracted only once per binary version.
func extractEmbeddedLibusb() (string, error) {
	data, ok := embeddedLibusbBytes()
	if !ok {
		return "", fmt.Errorf("no embedded libusb for this platform")
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache dir: %w", err)
	}

	libDir := filepath.Join(cacheDir, "dbibackend")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])[:16]
	target := filepath.Join(libDir, "libusb-"+hashStr+libusbExt())

	// Reuse if the file exists with the expected size.
	if info, err := os.Stat(target); err == nil && info.Size() == int64(len(data)) {
		return target, nil
	}

	// Atomic write: temp file then rename.
	tmp, err := os.CreateTemp(libDir, "libusb-*")
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write temp: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpPath, target); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename: %w", err)
	}

	return target, nil
}
