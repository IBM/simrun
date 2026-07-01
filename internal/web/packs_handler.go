package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/IBM/simrun/internal/config"
	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/packs/locks"
	"github.com/IBM/simrun/internal/packs/resolver"
	packrunner "github.com/IBM/simrun/internal/packs/runner"
	"github.com/go-chi/chi/v5"
)

// PackHandlers provides REST handlers for pack management.
type PackHandlers struct {
	packStore db.PackStore
	dataDir   string
}

// NewPackHandlers creates a new PackHandlers instance.
func NewPackHandlers(packStore db.PackStore, dataDir string) *PackHandlers {
	return &PackHandlers{packStore: packStore, dataDir: dataDir}
}

// HandleListPacks handles GET /api/packs
func (h *PackHandlers) HandleListPacks(w http.ResponseWriter, r *http.Request) {
	packs, err := h.packStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, packs)
}

// HandleInstallPack handles POST /api/packs/install.
//
// Install is an eager operation: the pack binary is made available (downloaded
// and checksum-verified for remote, verified on disk for local), its manifest
// command is run to derive the pack's identity, and only then is a row
// persisted. Any failure (bad repo, missing asset, checksum mismatch, manifest
// error, non-existent path) returns an error and creates no DB record. The
// request's `name` is ignored — the manifest is the source of truth.
func (h *PackHandlers) HandleInstallPack(w http.ResponseWriter, r *http.Request) {
	var req InstallPackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch config.PackType(req.Type) {
	case config.PackTypeLocal, config.PackTypeRemote:
		// installed below
	case config.PackTypeUpload:
		writeError(w, http.StatusBadRequest, "upload packs must be installed via POST /api/packs/upload")
		return
	default:
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("invalid pack type %q: allowed types are local, remote, upload", req.Type))
		return
	}

	res, err := resolver.NewResolver(h.dataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pack resolver")
		return
	}

	var pack *db.Pack
	if config.PackType(req.Type) == config.PackTypeRemote {
		pack, err = h.installRemote(r.Context(), res, req)
	} else {
		pack, err = h.installLocal(r.Context(), res, req)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.packStore.Upsert(r.Context(), pack, getUserEmail(r)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, pack)
}

// installRemote downloads + verifies + extracts a remote pack into a staging
// dir, runs its manifest to derive the name, then relocates the binary to the
// canonical version-keyed cache path. The persisted version is the resolved
// release tag (not an operator-typed value).
func (h *PackHandlers) installRemote(ctx context.Context, res *resolver.Resolver, req InstallPackRequest) (*db.Pack, error) {
	if req.Source == "" {
		return nil, fmt.Errorf("source is required for remote packs")
	}

	// Stage the download inside the cache dir so the later relocate is a
	// same-filesystem rename.
	staging, err := os.MkdirTemp(res.CacheDir(), ".staging-")
	if err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	fetched, err := res.Fetch(ctx, req.Source, req.Version, staging)
	if err != nil {
		return nil, err
	}

	manifest, err := res.GetManifest(ctx, fetched.BinaryPath)
	if err != nil {
		return nil, fmt.Errorf("manifest validation failed: %w", err)
	}
	name := manifest.Pack.Name
	if err := validateManifestName(name); err != nil {
		return nil, err
	}

	// Relocate under the per-pack lock so a concurrent install/delete of the
	// same pack cannot interleave with the rename.
	release := locks.Acquire(name)
	defer release()

	destPath := filepath.Join(res.CacheDir(), name, fetched.Version, filepath.Base(fetched.BinaryPath))
	if err := moveFile(fetched.BinaryPath, destPath); err != nil {
		return nil, err
	}

	return &db.Pack{
		Name:       name,
		Type:       string(config.PackTypeRemote),
		Source:     req.Source,
		Version:    fetched.Version,
		Status:     "installed",
		Parameters: req.Parameters,
	}, nil
}

// installLocal verifies the source path exists and runs its manifest to derive
// the pack's identity. The binary is referenced in place, never copied.
func (h *PackHandlers) installLocal(ctx context.Context, res *resolver.Resolver, req InstallPackRequest) (*db.Pack, error) {
	if req.Source == "" {
		return nil, fmt.Errorf("source (path) is required for local packs")
	}
	if _, err := os.Stat(req.Source); err != nil {
		return nil, fmt.Errorf("pack binary not found at %s: %w", req.Source, err)
	}

	manifest, err := res.GetManifest(ctx, req.Source)
	if err != nil {
		return nil, fmt.Errorf("manifest validation failed: %w", err)
	}
	name := manifest.Pack.Name
	if err := validateManifestName(name); err != nil {
		return nil, err
	}

	return &db.Pack{
		Name:       name,
		Type:       string(config.PackTypeLocal),
		Source:     req.Source,
		Version:    manifest.Pack.Version,
		Status:     "installed",
		Parameters: req.Parameters,
	}, nil
}

// validateManifestName rejects manifest names that are unsafe to use as a
// filesystem path component (the name becomes a cache directory for remote and
// upload packs).
func validateManifestName(name string) error {
	if name == "" {
		return fmt.Errorf("pack manifest did not report a name")
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") || name != filepath.Base(name) {
		return fmt.Errorf("invalid pack name %q from manifest: cannot contain path separators or '..'", name)
	}
	return nil
}

// moveFile relocates srcPath to destPath, creating the destination directory
// and replacing any existing file. Used to promote a staged binary to its
// canonical cache path; staging dirs live under the cache dir so this is a
// same-filesystem rename.
func moveFile(srcPath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	_ = os.Remove(destPath)
	if err := os.Rename(srcPath, destPath); err != nil {
		return fmt.Errorf("failed to relocate pack binary: %w", err)
	}
	return nil
}

// HandleDeletePack handles DELETE /api/packs/{name}
func (h *PackHandlers) HandleDeletePack(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "pack name is required")
		return
	}

	// Get pack before deletion to check if we need to clean up uploaded binary
	pack, _ := h.packStore.Get(r.Context(), name)

	if err := h.packStore.Delete(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Clean up uploaded binary from disk under the per-pack lock so the
	// RemoveAll cannot run concurrently with an install/upload of the same pack.
	if pack != nil && pack.Type == string(config.PackTypeUpload) {
		release := locks.Acquire(name)
		defer release()
		uploadDir := filepath.Join(h.dataDir, "packs", name, "upload")
		_ = os.RemoveAll(uploadDir)
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleGetManifest handles GET /api/packs/{name}/manifest
func (h *PackHandlers) HandleGetManifest(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "pack name is required")
		return
	}

	pack, err := h.packStore.Get(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "pack not found")
		return
	}

	factory, err := packrunner.NewFactory(h.dataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pack runner factory")
		return
	}

	cfg := config.PackConfig{
		Name:    pack.Name,
		Type:    config.PackType(pack.Type),
		Source:  pack.Source,
		Version: pack.Version,
	}

	manifest, err := factory.GetManifest(r.Context(), cfg, pack.Parameters, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, manifest)
}

// HandleGetPackParameters handles GET /api/packs/{name}/parameters
func (h *PackHandlers) HandleGetPackParameters(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "pack name is required")
		return
	}

	pack, err := h.packStore.Get(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "pack not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"parameters": pack.Parameters})
}

// HandleUpdatePackParameters handles PUT /api/packs/{name}/parameters.
// The request body is strict-validated against the pack's declared
// params_schema (fetched from the manifest). Declared keys must pass
// type, enum, and required-key checks; unknown keys are kept and
// surfaced in the response so the UI can render a soft warning.
// If the manifest fetch fails or the schema is empty, the handler
// falls back to permissive storage (today's behavior).
func (h *PackHandlers) HandleUpdatePackParameters(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "pack name is required")
		return
	}

	var req UpdatePackParametersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pack, err := h.packStore.Get(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "pack not found")
		return
	}

	schema := h.fetchPackParamsSchema(r, pack)
	errors, unknownKeys := validatePackParameters(schema, req.Parameters)
	if len(errors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  "parameter validation failed",
			"errors": errors,
		})
		return
	}

	if err := h.packStore.UpdateParameters(r.Context(), name, req.Parameters); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"parameters":   req.Parameters,
		"unknown_keys": unknownKeys,
	})
}

// fetchPackParamsSchema tries to fetch the pack's manifest and parse its
// params_schema. Returns nil on any failure so the caller can fall back
// to permissive storage.
func (h *PackHandlers) fetchPackParamsSchema(r *http.Request, pack *db.Pack) *paramSchema {
	factory, err := packrunner.NewFactory(h.dataDir)
	if err != nil {
		return nil
	}
	cfg := config.PackConfig{
		Name:    pack.Name,
		Type:    config.PackType(pack.Type),
		Source:  pack.Source,
		Version: pack.Version,
	}
	manifest, err := factory.GetManifest(r.Context(), cfg, pack.Parameters, nil)
	if err != nil || manifest == nil {
		return nil
	}
	schema, err := parsePackParamsSchema(manifest.ParamsSchema)
	if err != nil {
		return nil
	}
	return schema
}

// maxUploadSize is the maximum allowed pack binary upload size (500MB).
const maxUploadSize = 500 << 20

// HandleUploadPack handles POST /api/packs/upload.
//
// The uploaded binary is written to a staging location, its manifest command is
// run to derive the pack's identity, then it is relocated to
// <DataDir>/packs/<name>/upload/<name>. The request's `name` form field is
// ignored. A manifest failure aborts the install with no DB record.
func (h *PackHandlers) HandleUploadPack(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB memory buffer, rest goes to disk
		writeError(w, http.StatusBadRequest, "File too large (max 500MB) or invalid form data")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "File upload is required")
		return
	}
	defer file.Close()

	res, err := resolver.NewResolver(h.dataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pack resolver")
		return
	}

	// Stage the upload inside the cache dir so the later relocate is a
	// same-filesystem rename.
	staging, err := os.MkdirTemp(res.CacheDir(), ".staging-")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create staging directory")
		return
	}
	defer func() { _ = os.RemoveAll(staging) }()

	stagedPath := filepath.Join(staging, "pack")
	outFile, err := os.OpenFile(stagedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create binary file")
		return
	}
	if _, err := io.Copy(outFile, file); err != nil { //nolint:gosec // size-capped by MaxBytesReader above
		outFile.Close()
		writeError(w, http.StatusInternalServerError, "Failed to write binary file")
		return
	}
	if err := outFile.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to finalize binary file")
		return
	}

	// Derive identity from the manifest before persisting anything.
	manifest, err := res.GetManifest(r.Context(), stagedPath)
	if err != nil {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("Invalid pack binary: manifest validation failed: %v", err))
		return
	}
	name := manifest.Pack.Name
	if err := validateManifestName(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Serialize relocate + upsert against any concurrent upload or delete of the
	// same pack so they cannot interleave.
	release := locks.Acquire(name)
	defer release()

	// Replace any previous upload of this pack, then relocate the staged binary.
	uploadDir := filepath.Join(res.CacheDir(), name, "upload")
	if err := os.RemoveAll(uploadDir); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to clear previous upload")
		return
	}
	binaryPath := filepath.Join(uploadDir, name)
	if err := moveFile(stagedPath, binaryPath); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pack := &db.Pack{
		Name:    name,
		Type:    string(config.PackTypeUpload),
		Source:  binaryPath,
		Version: manifest.Pack.Version,
		Status:  "installed",
	}

	if err := h.packStore.Upsert(r.Context(), pack, getUserEmail(r)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, pack)
}
