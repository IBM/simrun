package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// platformArchiveName builds a goreleaser-style archive name for the test
// platform.
func platformArchiveName(prefix, version string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", prefix, version, runtime.GOOS, runtime.GOARCH)
}

// stubConfig configures the fake GitHub Releases server.
type stubConfig struct {
	latestTag     string            // tag returned for /releases/latest
	releases      map[string]bool   // tags that exist (e.g. "v1.2.3")
	assets        []githubAsset     // asset list returned for any existing release
	downloads     map[string][]byte // asset name -> body served on download
	checksumsBody string            // body served for checksums.txt
}

// startStub starts a server emulating the GitHub Releases API + asset downloads
// for a single repo. Asset download URLs point back at the server.
func startStub(t *testing.T, cfg stubConfig) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/releases/latest"):
			if cfg.latestTag == "" {
				http.NotFound(w, r)
				return
			}
			writeReleaseJSON(w, srv.URL, cfg.latestTag, cfg.assets)
		case strings.Contains(path, "/releases/tags/"):
			tag := path[strings.LastIndex(path, "/")+1:]
			if !cfg.releases[tag] {
				http.NotFound(w, r)
				return
			}
			writeReleaseJSON(w, srv.URL, tag, cfg.assets)
		case strings.HasSuffix(path, "/checksums.txt"):
			_, _ = w.Write([]byte(cfg.checksumsBody))
		case strings.HasPrefix(path, "/dl/"):
			name := path[strings.LastIndex(path, "/")+1:]
			body, ok := cfg.downloads[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func checksumsFor(names map[string][]byte) string {
	var b strings.Builder
	for name, body := range names {
		sum := sha256.Sum256(body)
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	return b.String()
}

func newTestResolver(t *testing.T, apiBaseURL string) *Resolver {
	t.Helper()
	r, err := NewResolverWithCacheDir(t.TempDir())
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	r.apiBaseURL = apiBaseURL
	return r
}

// TestFetch_PinnedVersion resolves an explicit version tag, verifies the
// checksum, and extracts the binary named after the asset prefix.
func TestFetch_PinnedVersion(t *testing.T) {
	const prefix, version = "mypack", "1.2.3"
	archiveName := platformArchiveName(prefix, version)
	binaryContent := []byte("#!/bin/sh\necho hi\n")
	archive := buildTarGz(t, prefix, binaryContent)

	srv := startStub(t, stubConfig{
		releases:      map[string]bool{"v" + version: true},
		assets:        []githubAsset{{Name: archiveName}, {Name: "checksums.txt"}},
		downloads:     map[string][]byte{archiveName: archive},
		checksumsBody: checksumsFor(map[string][]byte{archiveName: archive}),
	})
	r := newTestResolver(t, srv.URL)

	dest := t.TempDir()
	res, err := r.Fetch(context.Background(), "github.com/org/repo", version, dest)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Version != version {
		t.Fatalf("Version = %q, want %q", res.Version, version)
	}
	if filepath.Base(res.BinaryPath) != prefix {
		t.Fatalf("binary base = %q, want %q", filepath.Base(res.BinaryPath), prefix)
	}
	got, err := os.ReadFile(res.BinaryPath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(got) != string(binaryContent) {
		t.Fatalf("binary content mismatch")
	}
}

// TestFetch_VPrefixedInputEquivalent verifies that "v1.2.3" and "1.2.3" both
// resolve, so an operator can paste the tag exactly as GitHub displays it.
func TestFetch_VPrefixedInputEquivalent(t *testing.T) {
	const prefix, version = "mypack", "1.2.3"
	archiveName := platformArchiveName(prefix, version)
	archive := buildTarGz(t, prefix, []byte("bin"))
	stub := stubConfig{
		releases:      map[string]bool{"v" + version: true},
		assets:        []githubAsset{{Name: archiveName}, {Name: "checksums.txt"}},
		downloads:     map[string][]byte{archiveName: archive},
		checksumsBody: checksumsFor(map[string][]byte{archiveName: archive}),
	}

	for _, input := range []string{"1.2.3", "v1.2.3"} {
		srv := startStub(t, stub)
		r := newTestResolver(t, srv.URL)
		res, err := r.Fetch(context.Background(), "github.com/org/repo", input, t.TempDir())
		if err != nil {
			t.Fatalf("Fetch(%q): %v", input, err)
		}
		if res.Version != version {
			t.Fatalf("Fetch(%q) Version = %q, want %q", input, res.Version, version)
		}
	}
}

// TestFetch_BareTagFallback resolves a repo whose tags are not v-prefixed by
// falling back to the bare tag after the v-prefixed lookup 404s.
func TestFetch_BareTagFallback(t *testing.T) {
	const prefix, version = "mypack", "1.2.3"
	archiveName := platformArchiveName(prefix, version)
	archive := buildTarGz(t, prefix, []byte("bin"))
	srv := startStub(t, stubConfig{
		releases:      map[string]bool{version: true}, // tag "1.2.3", no leading v
		assets:        []githubAsset{{Name: archiveName}, {Name: "checksums.txt"}},
		downloads:     map[string][]byte{archiveName: archive},
		checksumsBody: checksumsFor(map[string][]byte{archiveName: archive}),
	})
	r := newTestResolver(t, srv.URL)

	res, err := r.Fetch(context.Background(), "github.com/org/repo", "v1.2.3", t.TempDir())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Version != version {
		t.Fatalf("Version = %q, want %q", res.Version, version)
	}
}

// TestFetch_LatestResolution resolves an empty version through /releases/latest
// and pins the concrete tag (leading "v" stripped).
func TestFetch_LatestResolution(t *testing.T) {
	const prefix, version = "mypack", "9.9.9"
	archiveName := platformArchiveName(prefix, version)
	archive := buildTarGz(t, prefix, []byte("bin"))

	srv := startStub(t, stubConfig{
		latestTag:     "v" + version,
		releases:      map[string]bool{"v" + version: true},
		assets:        []githubAsset{{Name: archiveName}, {Name: "checksums.txt"}},
		downloads:     map[string][]byte{archiveName: archive},
		checksumsBody: checksumsFor(map[string][]byte{archiveName: archive}),
	})
	r := newTestResolver(t, srv.URL)

	res, err := r.Fetch(context.Background(), "github.com/org/repo", "", t.TempDir())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Version != version {
		t.Fatalf("Version = %q, want %q (concrete tag, v stripped)", res.Version, version)
	}
}

// TestFetch_NoMatchingAsset fails when no asset matches the platform suffix.
func TestFetch_NoMatchingAsset(t *testing.T) {
	srv := startStub(t, stubConfig{
		releases: map[string]bool{"v1.0.0": true},
		assets:   []githubAsset{{Name: "mypack_1.0.0_plan9_sparc.tar.gz"}, {Name: "checksums.txt"}},
	})
	r := newTestResolver(t, srv.URL)

	_, err := r.Fetch(context.Background(), "github.com/org/repo", "1.0.0", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no asset matching") {
		t.Fatalf("expected no-asset error, got %v", err)
	}
}

// TestFetch_MultipleMatchingAssets fails and lists the candidates.
func TestFetch_MultipleMatchingAssets(t *testing.T) {
	a1 := platformArchiveName("mypack", "1.0.0")
	a2 := platformArchiveName("other", "1.0.0")
	srv := startStub(t, stubConfig{
		releases: map[string]bool{"v1.0.0": true},
		assets:   []githubAsset{{Name: a1}, {Name: a2}, {Name: "checksums.txt"}},
	})
	r := newTestResolver(t, srv.URL)

	_, err := r.Fetch(context.Background(), "github.com/org/repo", "1.0.0", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "multiple assets match") {
		t.Fatalf("expected multiple-asset error, got %v", err)
	}
	if !strings.Contains(err.Error(), a1) || !strings.Contains(err.Error(), a2) {
		t.Fatalf("error should list candidates, got %v", err)
	}
}

// TestFetch_ChecksumMismatch fails when the archive does not match checksums.txt.
func TestFetch_ChecksumMismatch(t *testing.T) {
	const prefix, version = "mypack", "1.0.0"
	archiveName := platformArchiveName(prefix, version)
	archive := buildTarGz(t, prefix, []byte("real"))

	srv := startStub(t, stubConfig{
		releases:      map[string]bool{"v" + version: true},
		assets:        []githubAsset{{Name: archiveName}, {Name: "checksums.txt"}},
		downloads:     map[string][]byte{archiveName: archive},
		checksumsBody: checksumsFor(map[string][]byte{archiveName: []byte("different bytes")}),
	})
	r := newTestResolver(t, srv.URL)

	_, err := r.Fetch(context.Background(), "github.com/org/repo", version, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

// TestFetch_RepoOrTagNotFound maps a 404 to a clear error.
func TestFetch_RepoOrTagNotFound(t *testing.T) {
	srv := startStub(t, stubConfig{releases: map[string]bool{}})
	r := newTestResolver(t, srv.URL)

	_, err := r.Fetch(context.Background(), "github.com/org/repo", "1.0.0", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// TestFetch_SingleExecutableFallback extracts the sole executable when no entry
// matches the asset prefix.
func TestFetch_SingleExecutableFallback(t *testing.T) {
	const version = "1.0.0"
	// Asset prefix is "weird" but the binary inside is named "actualbin"; a
	// non-executable README is also present.
	archiveName := platformArchiveName("weird", version)
	archive := buildTarGzMulti(t, []tarEntry{
		{name: "README.md", content: []byte("readme"), mode: 0644},
		{name: "actualbin", content: []byte("the binary"), mode: 0755},
	})

	srv := startStub(t, stubConfig{
		releases:      map[string]bool{"v" + version: true},
		assets:        []githubAsset{{Name: archiveName}, {Name: "checksums.txt"}},
		downloads:     map[string][]byte{archiveName: archive},
		checksumsBody: checksumsFor(map[string][]byte{archiveName: archive}),
	})
	r := newTestResolver(t, srv.URL)

	res, err := r.Fetch(context.Background(), "github.com/org/repo", version, t.TempDir())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if filepath.Base(res.BinaryPath) != "actualbin" {
		t.Fatalf("binary base = %q, want fallback to single executable %q", filepath.Base(res.BinaryPath), "actualbin")
	}
}

// TestFetch_AmbiguousBinaryFails errors when neither the prefix matches nor a
// single executable can be identified.
func TestFetch_AmbiguousBinaryFails(t *testing.T) {
	const version = "1.0.0"
	archiveName := platformArchiveName("weird", version)
	archive := buildTarGzMulti(t, []tarEntry{
		{name: "binA", content: []byte("a"), mode: 0755},
		{name: "binB", content: []byte("b"), mode: 0755},
	})

	srv := startStub(t, stubConfig{
		releases:      map[string]bool{"v" + version: true},
		assets:        []githubAsset{{Name: archiveName}, {Name: "checksums.txt"}},
		downloads:     map[string][]byte{archiveName: archive},
		checksumsBody: checksumsFor(map[string][]byte{archiveName: archive}),
	})
	r := newTestResolver(t, srv.URL)

	_, err := r.Fetch(context.Background(), "github.com/org/repo", version, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "cannot determine pack binary") {
		t.Fatalf("expected ambiguous-binary error, got %v", err)
	}
}
