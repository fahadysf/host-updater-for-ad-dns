package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	repoOwner = "fahadysf"
	repoName  = "host-updater-for-ad-dns"
	apiBase   = "https://api.github.com"
)

// ghRelease represents a GitHub release from the API.
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// ghAsset represents a release asset.
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// fetchLatestRelease fetches the latest release metadata from GitHub.
func fetchLatestRelease() (*ghRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", apiBase, repoOwner, repoName)
	debugLog.Printf("Fetching latest release from %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dns-updater/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	debugLog.Printf("Latest release: %s (%d assets)", rel.TagName, len(rel.Assets))
	return &rel, nil
}

// releaseVersion strips a leading "v" from the tag name to get the comparable version string.
func releaseVersion(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// isNewerVersion compares two version strings in YYYYmmdd.HHMM.commitid format.
// Only the timestamp portion (YYYYmmdd.HHMM) is compared lexicographically.
func isNewerVersion(latest, current string) bool {
	// Extract timestamp portions (first two dot-separated segments)
	latestTS := versionTimestamp(latest)
	currentTS := versionTimestamp(current)
	debugLog.Printf("Version comparison: latest=%s current=%s", latestTS, currentTS)
	return latestTS > currentTS
}

// versionTimestamp extracts the YYYYmmdd.HHMM portion from a version string.
func versionTimestamp(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}

// fetchChecksums downloads and parses checksums.txt from a release.
func fetchChecksums(rel *ghRelease) (map[string]string, error) {
	var checksumURL string
	for _, a := range rel.Assets {
		if a.Name == "checksums.txt" {
			checksumURL = a.BrowserDownloadURL
			break
		}
	}
	if checksumURL == "" {
		return nil, fmt.Errorf("checksums.txt not found in release assets")
	}

	debugLog.Printf("Downloading checksums from %s", checksumURL)
	resp, err := http.Get(checksumURL)
	if err != nil {
		return nil, fmt.Errorf("downloading checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums download returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading checksums: %w", err)
	}

	checksums := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			checksums[fields[1]] = fields[0]
		}
	}
	debugLog.Printf("Parsed %d checksums", len(checksums))
	return checksums, nil
}

// assetName returns the expected release asset name for the current platform.
func assetName() string {
	name := fmt.Sprintf("dns-updater-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// downloadAsset downloads a release asset to a temp file in the same directory as the executable.
func downloadAsset(rel *ghRelease, name string) (string, error) {
	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == name {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return "", fmt.Errorf("asset %s not found in release", name)
	}

	debugLog.Printf("Downloading %s from %s", name, downloadURL)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("downloading asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("asset download returned %d", resp.StatusCode)
	}

	// Create temp file in the same directory as the current executable for atomic rename
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("getting executable path: %w", err)
	}
	dir := filepath.Dir(execPath)

	tmpFile, err := os.CreateTemp(dir, "dns-updater-update-*.tmp")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("writing temp file: %w", err)
	}

	debugLog.Printf("Downloaded to %s", tmpPath)
	return tmpPath, nil
}

// verifySHA256 computes the SHA256 hash of a file and compares it to the expected value.
func verifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actual, expected)
	}
	debugLog.Printf("Checksum verified: %s", actual)
	return nil
}

// selfUpdate checks for a newer release and replaces the current binary if one is found.
func selfUpdate(outputFormat string) {
	if Version == "dev" {
		debugLog.Printf("Skipping auto-update for dev build")
		return
	}

	rel, err := fetchLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Auto-update: failed to check for updates: %v\n", err)
		return
	}

	latestVer := releaseVersion(rel.TagName)
	if !isNewerVersion(latestVer, Version) {
		debugLog.Printf("Already up to date (current=%s, latest=%s)", Version, latestVer)
		if outputFormat == OutputPretty {
			fmt.Fprintf(os.Stderr, "Already up to date (v%s).\n", Version)
		}
		return
	}

	if outputFormat == OutputPretty {
		fmt.Fprintf(os.Stderr, "New version available: v%s (current: v%s). Updating...\n", latestVer, Version)
	}

	// Fetch checksums
	checksums, err := fetchChecksums(rel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Auto-update: %v\n", err)
		return
	}

	// Determine asset name and verify checksum exists
	asset := assetName()
	expectedHash, ok := checksums[asset]
	if !ok {
		fmt.Fprintf(os.Stderr, "Auto-update: no checksum for %s\n", asset)
		return
	}

	// Download
	tmpPath, err := downloadAsset(rel, asset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Auto-update: %v\n", err)
		return
	}

	// Verify checksum
	if err := verifySHA256(tmpPath, expectedHash); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Auto-update: %v\n", err)
		return
	}

	// Get current executable path and permissions
	execPath, err := os.Executable()
	if err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Auto-update: cannot determine executable path: %v\n", err)
		return
	}
	// Resolve symlinks to get the real path
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Auto-update: cannot resolve executable path: %v\n", err)
		return
	}

	info, err := os.Stat(execPath)
	if err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Auto-update: cannot stat executable: %v\n", err)
		return
	}

	// Set permissions on new binary to match old one
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Auto-update: cannot set permissions: %v\n", err)
		return
	}

	// Atomic replace
	if runtime.GOOS == "windows" {
		// Windows locks running executables; rename current to .old first
		oldPath := execPath + ".old"
		os.Remove(oldPath) // ignore error if .old doesn't exist
		if err := os.Rename(execPath, oldPath); err != nil {
			os.Remove(tmpPath)
			fmt.Fprintf(os.Stderr, "Auto-update: cannot move old binary: %v\n", err)
			return
		}
		if err := os.Rename(tmpPath, execPath); err != nil {
			// Try to restore old binary
			os.Rename(oldPath, execPath)
			os.Remove(tmpPath)
			fmt.Fprintf(os.Stderr, "Auto-update: cannot install new binary: %v\n", err)
			return
		}
		// Clean up .old on best-effort basis
		os.Remove(oldPath)
	} else {
		if err := os.Rename(tmpPath, execPath); err != nil {
			os.Remove(tmpPath)
			fmt.Fprintf(os.Stderr, "Auto-update: cannot replace binary: %v\n", err)
			return
		}
	}

	if outputFormat == OutputPretty {
		fmt.Fprintf(os.Stderr, "Updated to v%s successfully.\n", latestVer)
	}
	debugLog.Printf("Updated from %s to %s", Version, latestVer)
}
