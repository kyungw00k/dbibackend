package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Release represents a GitHub release.
type Release struct {
	TagName string         `json:"tag_name"`
	Name    string         `json:"name"`
	Assets  []ReleaseAsset `json:"assets"`
}

// ReleaseAsset represents a single downloadable asset in a GitHub release.
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Updater handles checking for and applying updates.
type Updater struct {
	currentVersion string
	owner          string
	repo           string
	logger         *slog.Logger
	httpClient     *http.Client
}

// NewUpdater creates a new Updater for the given current version.
func NewUpdater(version string, logger *slog.Logger) *Updater {
	return &Updater{
		currentVersion: version,
		owner:          "kyungw00k",
		repo:           "dbibackend",
		logger:         logger,
		httpClient:     &http.Client{Timeout: 30 * defaultTimeoutUnit},
	}
}

const defaultTimeoutUnit = 1e9 // 1 second in nanoseconds

// Check queries the GitHub API for the latest release and returns it if newer.
func (u *Updater) Check() (*Release, bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", u.owner, u.repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "dbibackend-update-check")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil // no releases yet
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, false, fmt.Errorf("rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, fmt.Errorf("parse response: %w", err)
	}

	if !isNewer(u.currentVersion, release.TagName) {
		return &release, false, nil
	}

	return &release, true, nil
}

// Download downloads the platform-matching asset and verifies its checksum.
// Returns the path to the downloaded archive.
func (u *Updater) Download(release *Release) (string, error) {
	asset, err := selectAsset(release.Assets)
	if err != nil {
		return "", err
	}

	// Download the asset
	tmpDir, err := os.MkdirTemp("", "dbibackend-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := u.downloadFile(asset.BrowserDownloadURL, archivePath); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("download asset: %w", err)
	}

	// Find and download checksums.txt
	checksumsAsset := findChecksumsAsset(release.Assets)
	if checksumsAsset == nil {
		// No checksums file — skip verification (older releases)
		u.logger.Warn("no checksums.txt found, skipping verification")
		return archivePath, nil
	}

	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	if err := u.downloadFile(checksumsAsset.BrowserDownloadURL, checksumsPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("download checksums: %w", err)
	}

	// Verify SHA256
	if err := verifyChecksum(archivePath, asset.Name, checksumsPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("checksum verification: %w", err)
	}

	return archivePath, nil
}

// Apply extracts the binary from the archive and replaces the current executable.
func (u *Updater) Apply(archivePath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	// Extract the binary from the archive
	tmpDir := filepath.Dir(archivePath)
	var newExe string

	if runtime.GOOS == "windows" {
		newExe, err = extractFromZip(archivePath, tmpDir)
	} else {
		newExe, err = extractFromTarGz(archivePath, tmpDir)
	}
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	// Replace the current binary
	if runtime.GOOS == "windows" {
		return u.replaceWindows(exePath, newExe)
	}
	return u.replaceUnix(exePath, newExe)
}

// Restart starts a new instance of the application and the caller should exit.
func Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}
	return exec.Command(exe, os.Args[1:]...).Start()
}

// --- Version comparison ---

func isNewer(current, latest string) bool {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	if current == "dev" {
		return true
	}

	cur := parseSemver(current)
	lat := parseSemver(latest)
	if cur[0] < 0 || lat[0] < 0 {
		return false
	}

	return lat[0] > cur[0] ||
		(lat[0] == cur[0] && lat[1] > cur[1]) ||
		(lat[0] == cur[0] && lat[1] == cur[1] && lat[2] > cur[2])
}

func parseSemver(s string) [3]int {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return [3]int{-1, -1, -1}
	}
	var result [3]int
	for i, p := range parts {
		if idx := strings.Index(p, "-"); idx >= 0 {
			p = p[:idx]
		}
		n, err := fmt.Sscanf(p, "%d", &result[i])
		if err != nil || n != 1 {
			return [3]int{-1, -1, -1}
		}
	}
	return result
}

// --- Asset selection ---

func selectAsset(assets []ReleaseAsset) (*ReleaseAsset, error) {
	target := fmt.Sprintf("dbibackend_%s_%s.", runtime.GOOS, runtime.GOARCH)
	for i := range assets {
		if strings.HasPrefix(assets[i].Name, target) {
			return &assets[i], nil
		}
	}
	return nil, fmt.Errorf("no download for %s/%s", runtime.GOOS, runtime.GOARCH)
}

func findChecksumsAsset(assets []ReleaseAsset) *ReleaseAsset {
	for i := range assets {
		if assets[i].Name == "checksums.txt" {
			return &assets[i]
		}
	}
	return nil
}

// --- Download helpers ---

func (u *Updater) downloadFile(url, dest string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "dbibackend-update-check")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// --- Checksum verification ---

func verifyChecksum(filePath, assetName, checksumsPath string) error {
	// Read checksums.txt and find the entry for our asset
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}

	expectedHash := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) == 2 && (parts[1] == assetName || parts[1] == "./"+assetName) {
			expectedHash = parts[0]
			break
		}
	}
	if expectedHash == "" {
		return fmt.Errorf("no checksum entry for %s", assetName)
	}

	// Compute SHA256 of downloaded file
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// --- Archive extraction ---

func extractFromTarGz(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		name := filepath.Base(hdr.Name)
		if name != "dbibackend" || hdr.Typeflag != tar.TypeReg {
			continue
		}

		outPath := filepath.Join(destDir, "dbibackend-new")
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return "", err
		}
		defer out.Close()

		if _, err := io.Copy(out, tr); err != nil {
			return "", err
		}
		return outPath, nil
	}

	return "", fmt.Errorf("dbibackend binary not found in archive")
}

func extractFromZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name != "dbibackend.exe" || f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		outPath := filepath.Join(destDir, "dbibackend-new.exe")
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return "", err
		}
		defer out.Close()

		if _, err := io.Copy(out, rc); err != nil {
			return "", err
		}
		return outPath, nil
	}

	return "", fmt.Errorf("dbibackend.exe not found in archive")
}

// --- Binary replacement ---

func (u *Updater) replaceUnix(exePath, newExe string) error {
	backup := exePath + ".bak"

	// Move current binary to backup
	if err := os.Rename(exePath, backup); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	// Move new binary into place
	if err := os.Rename(newExe, exePath); err != nil {
		// Rollback
		os.Rename(backup, exePath)
		return fmt.Errorf("replace binary: %w", err)
	}

	// Ensure executable permission
	os.Chmod(exePath, 0755)

	// Clean up backup
	os.Remove(backup)

	return nil
}

func (u *Updater) replaceWindows(exePath, newExe string) error {
	// On Windows, the running binary is locked.
	// Write a batch script that waits for exit, then replaces.
	newPath := exePath + ".new"
	if err := os.Rename(newExe, newPath); err != nil {
		return fmt.Errorf("stage new binary: %w", err)
	}

	batch := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak >nul
del /f "%s"
ren "%s" "%s"
start "" "%s"
del "%%~f0"
`, exePath, newPath, filepath.Base(exePath), exePath)

	batchPath := filepath.Join(filepath.Dir(exePath), "update.bat")
	if err := os.WriteFile(batchPath, []byte(batch), 0644); err != nil {
		return fmt.Errorf("write batch script: %w", err)
	}

	return exec.Command("cmd", "/C", batchPath).Start()
}
