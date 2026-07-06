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
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// TestResolveConcurrentSamePackDownloadsOnce verifies that two concurrent
// Resolve calls for the same pack serialize so the archive is downloaded
// exactly once and the cached binary is complete. Without the per-pack lock
// both calls would download and O_TRUNC-write the same path, risking a torn
// binary observed by a reader.
func TestResolveConcurrentSamePackDownloadsOnce(t *testing.T) {
	const (
		packName = "demopack"
		version  = "1.2.3"
	)
	binaryContent := []byte("#!/bin/sh\necho complete-binary\n")
	archiveName := fmt.Sprintf("%s_%s_%s_%s.tar.gz", packName, version, runtime.GOOS, runtime.GOARCH)

	archive := buildTarGz(t, packName, binaryContent)
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archiveName)

	var archiveDownloads int32
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/tags/v"+version):
			writeReleaseJSON(w, srv.URL, "v"+version, []githubAsset{
				{Name: archiveName},
				{Name: "checksums.txt"},
			})
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			_, _ = w.Write([]byte(checksums))
		case strings.HasSuffix(r.URL.Path, archiveName):
			atomic.AddInt32(&archiveDownloads, 1)
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	r, err := NewResolverWithCacheDir(t.TempDir())
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	r.apiBaseURL = srv.URL

	cfg := PackConfig{Name: packName, Source: "github.com/org/repo", Version: version}

	var wg sync.WaitGroup
	paths := make([]string, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			paths[idx], errs[idx] = r.Resolve(context.Background(), cfg)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Resolve #%d failed: %v", i, err)
		}
	}

	if got := atomic.LoadInt32(&archiveDownloads); got != 1 {
		t.Fatalf("archive downloaded %d times; the per-pack lock should dedupe to 1", got)
	}

	got, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read cached binary: %v", err)
	}
	if !bytes.Equal(got, binaryContent) {
		t.Fatalf("cached binary is incomplete/torn: got %q want %q", got, binaryContent)
	}
}

// writeReleaseJSON writes a GitHub Releases API response whose asset download
// URLs point back at baseURL (so the resolver's default HTTP client reaches the
// stub without any transport rewriting).
func writeReleaseJSON(w http.ResponseWriter, baseURL, tag string, assets []githubAsset) {
	for i := range assets {
		if assets[i].BrowserDownloadURL == "" {
			assets[i].BrowserDownloadURL = fmt.Sprintf("%s/dl/%s/%s", baseURL, tag, assets[i].Name)
		}
	}
	_ = json.NewEncoder(w).Encode(githubRelease{TagName: tag, Assets: assets})
}

// buildTarGz returns a .tar.gz archive containing a single entry named
// binaryName with the given content.
func buildTarGz(t *testing.T, binaryName string, content []byte) []byte {
	t.Helper()
	return buildTarGzMulti(t, []tarEntry{{name: binaryName, content: content, mode: 0755}})
}

// tarEntry describes a single file written into a test archive.
type tarEntry struct {
	name    string
	content []byte
	mode    int64
}

// buildTarGzMulti returns a .tar.gz archive containing the given entries.
func buildTarGzMulti(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: e.mode, Size: int64(len(e.content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(e.content); err != nil {
			t.Fatalf("write tar content: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}
