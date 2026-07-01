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

	"github.com/IBM/simrun/internal/packs/locks"
	"github.com/IBM/simrun/pack"
	"github.com/sirupsen/logrus"
)

// defaultGitHubAPIBaseURL is the public GitHub Releases API base. It can be
// overridden via SR_GITHUB_API_URL (used by tests and, incidentally, for
// GitHub Enterprise).
const defaultGitHubAPIBaseURL = "https://api.github.com"

// PackConfig represents configuration for a remote pack to resolve.
type PackConfig struct {
	Name    string
	Source  string
	Version string
}

// FetchResult is returned by Fetch after a remote pack has been downloaded,
// verified, and extracted.
type FetchResult struct {
	// Version is the concrete resolved release tag with any leading "v" stripped.
	Version string
	// BinaryPath is the absolute path to the extracted pack binary.
	BinaryPath string
}

// Resolver downloads, caches, and resolves pack binaries.
type Resolver struct {
	cacheDir   string
	httpClient *http.Client
	apiBaseURL string
}

// NewResolver creates a new Resolver caching packs under <dataDir>/packs/.
func NewResolver(dataDir string) (*Resolver, error) {
	return NewResolverWithCacheDir(filepath.Join(dataDir, "packs"))
}

// NewResolverWithCacheDir creates a new Resolver with a custom cache directory.
func NewResolverWithCacheDir(cacheDir string) (*Resolver, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	apiBaseURL := defaultGitHubAPIBaseURL
	if v := os.Getenv("SR_GITHUB_API_URL"); v != "" {
		apiBaseURL = strings.TrimRight(v, "/")
	}

	return &Resolver{
		cacheDir:   cacheDir,
		httpClient: &http.Client{},
		apiBaseURL: apiBaseURL,
	}, nil
}

// CacheDir returns the cache directory path.
func (r *Resolver) CacheDir() string {
	return r.cacheDir
}

// Resolve returns the path to a remote pack binary, downloading if needed.
// At runtime the binary is normally already cached (populated at install), so
// this is a cache hit; on a miss it re-fetches the pinned release.
func (r *Resolver) Resolve(ctx context.Context, cfg PackConfig) (string, error) {
	if err := r.validatePackConfig(cfg); err != nil {
		return "", err
	}

	if cachedPath, ok := r.cachedBinary(cfg); ok {
		logrus.WithField("pack", cfg.Name).WithField("version", cfg.Version).WithField("path", cachedPath).Debug("Using cached pack binary")
		return cachedPath, nil
	}

	// Serialize downloads of the same pack across all resolver instances so a
	// concurrent download cannot O_TRUNC-write the same binary mid-read.
	release := locks.Acquire(cfg.Name)
	defer release()

	// Double-checked: another waiter may have downloaded it while we blocked.
	if cachedPath, ok := r.cachedBinary(cfg); ok {
		logrus.WithField("pack", cfg.Name).WithField("version", cfg.Version).WithField("path", cachedPath).Debug("Using cached pack binary")
		return cachedPath, nil
	}

	destDir := filepath.Join(r.cacheDir, cfg.Name, cfg.Version)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	res, err := r.Fetch(ctx, cfg.Source, cfg.Version, destDir)
	if err != nil {
		return "", fmt.Errorf("failed to download pack %s: %w", cfg.Name, err)
	}

	logrus.WithField("pack", cfg.Name).WithField("version", res.Version).WithField("path", res.BinaryPath).Info("Pack downloaded and cached")
	return res.BinaryPath, nil
}

// validatePackConfig validates the pack configuration.
func (r *Resolver) validatePackConfig(cfg PackConfig) error {
	if cfg.Source == "" {
		return fmt.Errorf("pack %s: source is required", cfg.Name)
	}
	return nil
}

// cachedBinary returns the path to the cached binary for cfg, if present. The
// binary's on-disk name is the manifest-derived name picked at install time, so
// it is located by scanning the version directory rather than assuming a name.
func (r *Resolver) cachedBinary(cfg PackConfig) (string, bool) {
	dir := filepath.Join(r.cacheDir, cfg.Name, cfg.Version)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return filepath.Join(dir, e.Name()), true
		}
	}
	return "", false
}

// GetManifest calls the pack's manifest command and parses the response. This
// is the shared helper used at install time to derive a pack's identity.
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

// Fetch resolves a remote pack release via the GitHub Releases API, downloads
// and checksum-verifies the platform archive, and extracts the binary into
// destDir. version may be empty to resolve the latest release. It returns the
// concrete resolved version and the extracted binary path.
func (r *Resolver) Fetch(ctx context.Context, source, version, destDir string) (FetchResult, error) {
	org, repo, err := parseGitHubSource(source)
	if err != nil {
		return FetchResult{}, err
	}

	rel, err := r.resolveRelease(ctx, org, repo, version)
	if err != nil {
		return FetchResult{}, err
	}

	archive, checksums, err := selectAssets(rel.Assets, rel.TagName)
	if err != nil {
		return FetchResult{}, err
	}

	logrus.WithField("source", source).WithField("tag", rel.TagName).WithField("asset", archive.Name).Info("Downloading pack")

	archiveData, err := r.downloadAndVerify(ctx, archive, checksums)
	if err != nil {
		return FetchResult{}, err
	}

	binaryName, err := extractBinary(archiveData, destDir, binaryNameFromAsset(archive.Name))
	if err != nil {
		return FetchResult{}, fmt.Errorf("failed to extract archive: %w", err)
	}

	return FetchResult{
		Version:    strings.TrimPrefix(rel.TagName, "v"),
		BinaryPath: filepath.Join(destDir, binaryName),
	}, nil
}

// githubRelease is the subset of the GitHub Releases API response we use.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset is a single release asset.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// resolveRelease fetches release metadata from the GitHub Releases API. An
// empty version resolves the latest release. Otherwise the version is treated
// as a bare version regardless of how the operator typed it ("v0.0.1" and
// "0.0.1" are equivalent): the conventional "v"-prefixed tag is tried first,
// then the bare tag, so both common tagging conventions resolve.
func (r *Resolver) resolveRelease(ctx context.Context, org, repo, version string) (*githubRelease, error) {
	if version == "" {
		rel, notFound, err := r.getRelease(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/latest", r.apiBaseURL, org, repo), org, repo)
		if err != nil {
			return nil, err
		}
		if notFound {
			return nil, fmt.Errorf("no published release found for %s/%s", org, repo)
		}
		return rel, nil
	}

	bare := strings.TrimPrefix(version, "v")
	for _, tag := range []string{"v" + bare, bare} {
		url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", r.apiBaseURL, org, repo, tag)
		rel, notFound, err := r.getRelease(ctx, url, org, repo)
		if err != nil {
			return nil, err
		}
		if !notFound {
			return rel, nil
		}
	}
	return nil, fmt.Errorf("release not found for %s/%s: no tag %q or %q exists", org, repo, "v"+bare, bare)
}

// getRelease performs a single Releases API GET. It reports notFound==true for a
// 404 (so the caller can try an alternative tag spelling) and returns a clear
// error for rate-limit and other non-OK responses.
func (r *Resolver) getRelease(ctx context.Context, url, org, repo string) (rel *githubRelease, notFound bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// proceed
	case http.StatusNotFound:
		return nil, true, nil
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, false, fmt.Errorf("GitHub API rate limit exceeded resolving %s/%s: retry later or pin an explicit version", org, repo)
	default:
		return nil, false, fmt.Errorf("GitHub API returned HTTP %d resolving %s/%s", resp.StatusCode, org, repo)
	}

	var decoded githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, false, fmt.Errorf("failed to parse GitHub release response: %w", err)
	}
	if decoded.TagName == "" {
		return nil, false, fmt.Errorf("GitHub release for %s/%s has no tag", org, repo)
	}
	return &decoded, false, nil
}

// selectAssets picks the platform archive (matching *_<os>_<arch>.tar.gz) and
// the checksums asset from a release's asset list.
func selectAssets(assets []githubAsset, tag string) (archive, checksums githubAsset, err error) {
	suffix := fmt.Sprintf("_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	var matches []githubAsset
	for _, a := range assets {
		if strings.HasSuffix(a.Name, suffix) {
			matches = append(matches, a)
		}
	}

	switch len(matches) {
	case 0:
		return githubAsset{}, githubAsset{}, fmt.Errorf("no asset matching *%s for %s/%s in release %s", suffix, runtime.GOOS, runtime.GOARCH, tag)
	case 1:
		archive = matches[0]
	default:
		names := make([]string, len(matches))
		for i, a := range matches {
			names[i] = a.Name
		}
		return githubAsset{}, githubAsset{}, fmt.Errorf("multiple assets match *%s in release %s: %s", suffix, tag, strings.Join(names, ", "))
	}

	checksums, ok := findChecksumsAsset(assets)
	if !ok {
		return githubAsset{}, githubAsset{}, fmt.Errorf("no checksums.txt asset found in release %s", tag)
	}
	return archive, checksums, nil
}

// findChecksumsAsset locates the checksums asset, preferring an exact
// "checksums.txt" name and falling back to any asset ending in "checksums.txt".
func findChecksumsAsset(assets []githubAsset) (githubAsset, bool) {
	for _, a := range assets {
		if a.Name == "checksums.txt" {
			return a, true
		}
	}
	for _, a := range assets {
		if strings.HasSuffix(a.Name, "checksums.txt") {
			return a, true
		}
	}
	return githubAsset{}, false
}

// binaryNameFromAsset derives the expected in-tarball binary name from an
// archive asset name of the form <name>_<version>_<os>_<arch>.tar.gz. This is a
// best-effort hint; extraction falls back to the single executable in the
// archive when no entry matches.
func binaryNameFromAsset(assetName string) string {
	return strings.SplitN(assetName, "_", 2)[0]
}

// downloadAndVerify downloads an archive asset and verifies its checksum against
// the release's checksums asset.
func (r *Resolver) downloadAndVerify(ctx context.Context, archive, checksumsAsset githubAsset) ([]byte, error) {
	checksums, err := r.downloadChecksums(ctx, checksumsAsset.BrowserDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download checksums: %w", err)
	}

	expectedChecksum, ok := checksums[archive.Name]
	if !ok {
		return nil, fmt.Errorf("checksum not found for %s", archive.Name)
	}

	archiveData, err := r.downloadFile(ctx, archive.BrowserDownloadURL)
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

// extractBinary extracts the pack binary from a .tar.gz archive into destDir.
// It prefers the entry whose base name == preferredName (derived from the asset
// prefix); otherwise it falls back to the single executable in the archive, or
// the sole regular file. Returns the extracted binary's base name.
func extractBinary(data []byte, destDir, preferredName string) (string, error) {
	target, err := chooseArchiveBinary(data, preferredName)
	if err != nil {
		return "", err
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar entry: %w", err)
		}
		if header.Name != target {
			continue
		}

		baseName := filepath.Base(target)
		destPath := filepath.Join(destDir, baseName)
		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create file: %w", err)
		}
		if _, err := io.Copy(outFile, tarReader); err != nil { //nolint:gosec // archive is checksum-verified before extraction
			outFile.Close()
			return "", fmt.Errorf("failed to write file: %w", err)
		}
		if err := outFile.Close(); err != nil {
			return "", fmt.Errorf("failed to finalize file: %w", err)
		}
		return baseName, nil
	}

	return "", fmt.Errorf("binary %q not found in archive", target)
}

// chooseArchiveBinary scans the archive's headers and decides which entry is the
// pack binary, without reading entry contents.
func chooseArchiveBinary(data []byte, preferredName string) (string, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	var regularFiles []string
	var execFiles []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar entry: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if preferredName != "" && filepath.Base(header.Name) == preferredName {
			return header.Name, nil
		}
		regularFiles = append(regularFiles, header.Name)
		if header.Mode&0111 != 0 {
			execFiles = append(execFiles, header.Name)
		}
	}

	if len(execFiles) == 1 {
		return execFiles[0], nil
	}
	if len(regularFiles) == 1 {
		return regularFiles[0], nil
	}
	if len(regularFiles) == 0 {
		return "", fmt.Errorf("archive contains no files")
	}
	return "", fmt.Errorf("cannot determine pack binary in archive: %d candidate files and no unique executable (looked for %q)", len(regularFiles), preferredName)
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
