package web_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IBM/simrun/internal/testutil/testserver"
	"github.com/IBM/simrun/internal/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// manifestScript returns the bytes of a shell script that prints a manifest
// response reporting the given pack name and version.
func manifestScript(name, version string) []byte {
	return fmt.Appendf(nil, "#!/bin/sh\necho '{\"pack\":{\"name\":\"%s\",\"version\":\"%s\"},\"simulations\":[]}'\n", name, version)
}

// failingManifestScript returns a script whose manifest command exits non-zero.
func failingManifestScript() []byte {
	return []byte("#!/bin/sh\necho boom >&2\nexit 1\n")
}

// writeExecutable writes content as an executable file under dir and returns its path.
func writeExecutable(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, content, 0755))
	return p
}

// platformArchiveName builds a goreleaser-style archive name for the test platform.
func platformArchiveName(prefix, version string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", prefix, version, runtime.GOOS, runtime.GOARCH)
}

// tarGzWith builds a .tar.gz containing a single executable entry.
func tarGzWith(t *testing.T, binaryName string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: binaryName, Mode: 0755, Size: int64(len(content)), Typeflag: tar.TypeReg}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

type releaseJSON struct {
	TagName string      `json:"tag_name"`
	Assets  []assetJSON `json:"assets"`
}

type assetJSON struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// startGitHubStub emulates the GitHub Releases API + downloads for a single
// repo with one platform archive. latestTag is returned for /releases/latest;
// existingTags are the tags resolvable via /releases/tags/<tag>.
func startGitHubStub(t *testing.T, latestTag string, existingTags map[string]bool, archiveName string, archive []byte) *httptest.Server {
	t.Helper()
	checksums := func() string {
		sum := sha256.Sum256(archive)
		return fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archiveName)
	}()

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		writeRel := func(tag string) {
			_ = json.NewEncoder(w).Encode(releaseJSON{
				TagName: tag,
				Assets: []assetJSON{
					{Name: archiveName, BrowserDownloadURL: srv.URL + "/dl/" + archiveName},
					{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/dl/checksums.txt"},
				},
			})
		}
		switch {
		case strings.HasSuffix(path, "/releases/latest"):
			if latestTag == "" {
				http.NotFound(w, r)
				return
			}
			writeRel(latestTag)
		case strings.Contains(path, "/releases/tags/"):
			tag := path[strings.LastIndex(path, "/")+1:]
			if !existingTags[tag] {
				http.NotFound(w, r)
				return
			}
			writeRel(tag)
		case strings.HasSuffix(path, "/dl/checksums.txt"):
			_, _ = w.Write([]byte(checksums))
		case strings.HasSuffix(path, "/dl/"+archiveName):
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestInstallPack_RemotePinsResolvedTagAndManifestName verifies a remote install
// downloads, runs the manifest, ignores the request name, and pins the row's
// version to the resolved release tag.
func TestInstallPack_RemotePinsResolvedTagAndManifestName(t *testing.T) {
	const tag, version = "v2.3.4", "2.3.4"
	archiveName := platformArchiveName("realpack", version)
	// The in-tarball binary is named "realpack" but the manifest reports a
	// different identity: "real-pack". The row must take the manifest name.
	archive := tarGzWith(t, "realpack", manifestScript("real-pack", "manifest-version-ignored"))
	srv := startGitHubStub(t, "", map[string]bool{tag: true}, archiveName, archive)
	t.Setenv("SR_GITHUB_API_URL", srv.URL)

	ts := testserver.New(t)
	resp := ts.Post(t, "/api/packs/install", web.InstallPackRequest{
		Name:    "operator-typed",
		Type:    "remote",
		Source:  "github.com/org/repo",
		Version: version,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, testserver.ReadBody(t, resp))

	got, err := ts.Stores.Pack.Get(t.Context(), "real-pack")
	require.NoError(t, err, "row must be keyed by the manifest name, not the request name")
	assert.Equal(t, "2.3.4", got.Version, "version must be the resolved release tag")
	assert.Equal(t, "github.com/org/repo", got.Source)

	_, err = ts.Stores.Pack.Get(t.Context(), "operator-typed")
	assert.Error(t, err, "request name must be ignored")
}

// TestInstallPack_RemoteLatest resolves an omitted version to the latest tag.
func TestInstallPack_RemoteLatest(t *testing.T) {
	const tag, version = "v9.9.9", "9.9.9"
	archiveName := platformArchiveName("realpack", version)
	archive := tarGzWith(t, "realpack", manifestScript("latest-pack", "ignored"))
	srv := startGitHubStub(t, tag, map[string]bool{tag: true}, archiveName, archive)
	t.Setenv("SR_GITHUB_API_URL", srv.URL)

	ts := testserver.New(t)
	resp := ts.Post(t, "/api/packs/install", web.InstallPackRequest{
		Type:   "remote",
		Source: "github.com/org/repo",
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, testserver.ReadBody(t, resp))

	got, err := ts.Stores.Pack.Get(t.Context(), "latest-pack")
	require.NoError(t, err)
	assert.Equal(t, "9.9.9", got.Version, "latest must be pinned to the concrete tag")
}

// TestInstallPack_RemoteRepoNotFound creates no row when the release is missing.
func TestInstallPack_RemoteRepoNotFound(t *testing.T) {
	srv := startGitHubStub(t, "", map[string]bool{}, "x.tar.gz", nil)
	t.Setenv("SR_GITHUB_API_URL", srv.URL)

	ts := testserver.New(t)
	resp := ts.Post(t, "/api/packs/install", web.InstallPackRequest{
		Type:    "remote",
		Source:  "github.com/org/repo",
		Version: "1.0.0",
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	packs, err := ts.Stores.Pack.List(t.Context())
	require.NoError(t, err)
	assert.Empty(t, packs, "no DB row on resolution failure")
}

// TestInstallPack_LocalManifestDerived takes name + version from the manifest.
func TestInstallPack_LocalManifestDerived(t *testing.T) {
	bin := writeExecutable(t, t.TempDir(), "anything", manifestScript("local-pack", "0.7.0"))

	ts := testserver.New(t)
	resp := ts.Post(t, "/api/packs/install", web.InstallPackRequest{
		Name:   "ignored",
		Type:   "local",
		Source: bin,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, testserver.ReadBody(t, resp))

	got, err := ts.Stores.Pack.Get(t.Context(), "local-pack")
	require.NoError(t, err)
	assert.Equal(t, "0.7.0", got.Version, "local version comes from the manifest")
	assert.Equal(t, bin, got.Source, "local source is referenced in place")
}

// TestInstallPack_LocalNonExistentPath rejects a missing path with no row.
func TestInstallPack_LocalNonExistentPath(t *testing.T) {
	ts := testserver.New(t)
	resp := ts.Post(t, "/api/packs/install", web.InstallPackRequest{
		Type:   "local",
		Source: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	packs, err := ts.Stores.Pack.List(t.Context())
	require.NoError(t, err)
	assert.Empty(t, packs, "no DB row when the path does not exist")
}

// TestInstallPack_LocalManifestFailure rejects a binary whose manifest fails.
func TestInstallPack_LocalManifestFailure(t *testing.T) {
	bin := writeExecutable(t, t.TempDir(), "broken", failingManifestScript())

	ts := testserver.New(t)
	resp := ts.Post(t, "/api/packs/install", web.InstallPackRequest{
		Type:   "local",
		Source: bin,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	packs, err := ts.Stores.Pack.List(t.Context())
	require.NoError(t, err)
	assert.Empty(t, packs, "no DB row when the manifest command fails")
}

// TestInstallPack_UploadTypeRejectedOnInstall directs upload installs to the
// dedicated endpoint.
func TestInstallPack_UploadTypeRejectedOnInstall(t *testing.T) {
	ts := testserver.New(t)
	resp := ts.Post(t, "/api/packs/install", web.InstallPackRequest{Type: "upload", Source: "x"})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	packs, err := ts.Stores.Pack.List(t.Context())
	require.NoError(t, err)
	assert.Empty(t, packs)
}
