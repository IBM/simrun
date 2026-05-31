// Package resolver provides pack binary resolution and caching.
package resolver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/IBM/simrun/simrun/internal/packs/locks"
	"github.com/IBM/simrun/simrun/pack"
	"github.com/sirupsen/logrus"
)

// PackConfig represents configuration for a remote pack to resolve.
type PackConfig struct {
	Name    string
	Source  string
	Version string
}

// Resolver downloads, caches, and resolves pack binaries.
type Resolver struct {
	cacheDir   string
	httpClient *http.Client
}

// NewResolver creates a new Resolver caching packs under <dataDir>/packs/.
func NewResolver(dataDir string) (*Resolver, error) {
	cacheDir := filepath.Join(dataDir, "packs")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Resolver{
		cacheDir:   cacheDir,
		httpClient: &http.Client{},
	}, nil
}

// NewResolverWithCacheDir creates a new Resolver with a custom cache directory.
func NewResolverWithCacheDir(cacheDir string) (*Resolver, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Resolver{
		cacheDir:   cacheDir,
		httpClient: &http.Client{},
	}, nil
}

// CacheDir returns the cache directory path.
func (r *Resolver) CacheDir() string {
	return r.cacheDir
}

// Resolve returns the path to a remote pack binary, downloading if needed.
func (r *Resolver) Resolve(ctx context.Context, cfg PackConfig) (string, error) {
	if err := r.validatePackConfig(cfg); err != nil {
		return "", err
	}

	cachedPath := r.getCachedPath(cfg)
	if r.isCached(cachedPath) {
		logrus.WithField("pack", cfg.Name).WithField("version", cfg.Version).WithField("path", cachedPath).Debug("Using cached pack binary")
		return cachedPath, nil
	}

	// Serialize downloads of the same pack across all resolver instances so a
	// concurrent download cannot O_TRUNC-write the same binary mid-read.
	release := locks.Acquire(cfg.Name)
	defer release()

	// Double-checked: another waiter may have downloaded it while we blocked.
	if r.isCached(cachedPath) {
		logrus.WithField("pack", cfg.Name).WithField("version", cfg.Version).WithField("path", cachedPath).Debug("Using cached pack binary")
		return cachedPath, nil
	}

	if err := r.download(ctx, cfg); err != nil {
		return "", fmt.Errorf("failed to download pack %s: %w", cfg.Name, err)
	}

	return cachedPath, nil
}

// validatePackConfig validates the pack configuration.
func (r *Resolver) validatePackConfig(cfg PackConfig) error {
	if cfg.Source == "" {
		return fmt.Errorf("pack %s: source is required", cfg.Name)
	}
	if cfg.Version == "" {
		return fmt.Errorf("pack %s: version is required for remote packs", cfg.Name)
	}
	return nil
}

// isCached checks if a file exists at the given path.
func (r *Resolver) isCached(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetManifest calls the pack's manifest command and parses the response.
func (r *Resolver) GetManifest(ctx context.Context, packPath string) (*pack.ManifestResponse, error) {
	cmd := exec.CommandContext(ctx, packPath, "manifest")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("manifest command failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to run manifest command: %w", err)
	}

	var manifest pack.ManifestResponse
	if err := json.Unmarshal(output, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest response: %w", err)
	}

	return &manifest, nil
}

// getCachedPath returns the path where a pack binary should be cached.
func (r *Resolver) getCachedPath(cfg PackConfig) string {
	return filepath.Join(r.cacheDir, cfg.Name, cfg.Version, cfg.Name)
}

// download downloads a pack from GitHub releases.
func (r *Resolver) download(ctx context.Context, cfg PackConfig) error {
	org, repo, err := parseGitHubSource(cfg.Source)
	if err != nil {
		return err
	}

	urls := r.buildDownloadURLs(org, repo, cfg)
	logrus.WithField("pack", cfg.Name).WithField("version", cfg.Version).WithField("url", urls.archive).Info("Downloading pack")

	archiveData, err := r.downloadAndVerify(ctx, urls, cfg.Name)
	if err != nil {
		return err
	}

	if err := r.extractAndCache(archiveData, cfg); err != nil {
		return err
	}

	logrus.WithField("pack", cfg.Name).WithField("version", cfg.Version).WithField("path", r.getCachedPath(cfg)).Info("Pack downloaded and cached")
	return nil
}

// downloadURLs contains URLs for downloading a pack.
type downloadURLs struct {
	archive     string
	checksums   string
	archiveName string
}

// buildDownloadURLs builds the download URLs for a pack.
func (r *Resolver) buildDownloadURLs(org, repo string, cfg PackConfig) downloadURLs {
	archiveName := fmt.Sprintf("%s_%s_%s_%s.tar.gz", cfg.Name, cfg.Version, runtime.GOOS, runtime.GOARCH)
	baseURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/v%s", org, repo, cfg.Version)

	return downloadURLs{
		archive:     fmt.Sprintf("%s/%s", baseURL, archiveName),
		checksums:   fmt.Sprintf("%s/checksums.txt", baseURL),
		archiveName: archiveName,
	}
}

// downloadAndVerify downloads an archive and verifies its checksum.
func (r *Resolver) downloadAndVerify(ctx context.Context, urls downloadURLs, packName string) ([]byte, error) {
	checksums, err := r.downloadChecksums(ctx, urls.checksums)
	if err != nil {
		return nil, fmt.Errorf("failed to download checksums: %w", err)
	}

	expectedChecksum, ok := checksums[urls.archiveName]
	if !ok {
		return nil, fmt.Errorf("checksum not found for %s", urls.archiveName)
	}

	archiveData, err := r.downloadFile(ctx, urls.archive)
	if err != nil {
		return nil, fmt.Errorf("failed to download archive: %w", err)
	}

	if err := r.verifyChecksum(archiveData, expectedChecksum); err != nil {
		return nil, err
	}

	return archiveData, nil
}

// verifyChecksum verifies the checksum of data.
func (r *Resolver) verifyChecksum(data []byte, expected string) error {
	actual := sha256.Sum256(data)
	actualHex := hex.EncodeToString(actual[:])
	if actualHex != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actualHex)
	}
	return nil
}

// extractAndCache extracts an archive and caches it.
func (r *Resolver) extractAndCache(archiveData []byte, cfg PackConfig) error {
	cacheDir := filepath.Join(r.cacheDir, cfg.Name, cfg.Version)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := r.extractTarGz(archiveData, cacheDir, cfg.Name); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	return nil
}

// downloadFile downloads a file from a URL.
func (r *Resolver) downloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// downloadChecksums downloads and parses a checksums.txt file.
func (r *Resolver) downloadChecksums(ctx context.Context, url string) (map[string]string, error) {
	data, err := r.downloadFile(ctx, url)
	if err != nil {
		return nil, err
	}

	return r.parseChecksums(string(data))
}

// parseChecksums parses checksum data into a map.
func (r *Resolver) parseChecksums(data string) (map[string]string, error) {
	checksums := make(map[string]string)
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		checksum, filename := r.parseChecksumLine(line)
		if checksum != "" && filename != "" {
			checksums[filename] = checksum
		}
	}

	return checksums, nil
}

// parseChecksumLine parses a single checksum line.
func (r *Resolver) parseChecksumLine(line string) (checksum, filename string) {
	// Format: <checksum>  <filename> (two spaces between)
	parts := strings.SplitN(line, "  ", 2)
	if len(parts) != 2 {
		// Try single space
		parts = strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return "", ""
		}
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

// extractTarGz extracts a .tar.gz archive.
func (r *Resolver) extractTarGz(data []byte, destDir string, expectedBinaryName string) error {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Only extract the binary file
		baseName := filepath.Base(header.Name)
		if baseName != expectedBinaryName {
			continue
		}

		destPath := filepath.Join(destDir, expectedBinaryName)
		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return fmt.Errorf("failed to write file: %w", err)
		}
		outFile.Close()

		return nil
	}

	return fmt.Errorf("binary %s not found in archive", expectedBinaryName)
}

// parseGitHubSource parses a GitHub source string (e.g., "github.com/org/repo").
func parseGitHubSource(source string) (org, repo string, err error) {
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")

	parts := strings.Split(source, "/")
	if len(parts) != 3 || parts[0] != "github.com" {
		return "", "", fmt.Errorf("invalid GitHub source: %s (expected github.com/org/repo)", source)
	}

	return parts[1], parts[2], nil
}
