package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/IBM/simrun/simrun/internal/config"
	"github.com/IBM/simrun/simrun/internal/db"
	"github.com/IBM/simrun/simrun/internal/packs/locks"
	packrunner "github.com/IBM/simrun/simrun/internal/packs/runner"
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

// HandleInstallPack handles POST /api/packs/install
func (h *PackHandlers) HandleInstallPack(w http.ResponseWriter, r *http.Request) {
	var req InstallPackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch config.PackType(req.Type) {
	case config.PackTypeLocal, config.PackTypeRemote, config.PackTypeUpload:
		// recognized
	default:
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("invalid pack type %q: allowed types are local, remote, upload", req.Type))
		return
	}

	pack := &db.Pack{
		Name:       req.Name,
		Type:       req.Type,
		Source:     req.Source,
		Version:    req.Version,
		Status:     "installed",
		Parameters: req.Parameters,
	}

	if err := h.packStore.Upsert(r.Context(), pack, getUserEmail(r)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, pack)
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

// HandleUploadPack handles POST /api/packs/upload
func (h *PackHandlers) HandleUploadPack(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB memory buffer, rest goes to disk
		writeError(w, http.StatusBadRequest, "File too large (max 500MB) or invalid form data")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Pack name is required")
		return
	}

	// Validate name to prevent directory traversal
	if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") || name != filepath.Base(name) {
		writeError(w, http.StatusBadRequest, "Invalid pack name: cannot contain path separators or '..'")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "File upload is required")
		return
	}
	defer file.Close()

	// Serialize the write + manifest-validate + upsert against any concurrent
	// upload or delete of the same pack so they cannot interleave mid-write.
	release := locks.Acquire(name)
	defer release()

	// Determine upload directory
	uploadDir := filepath.Join(h.dataDir, "packs", name, "upload")
	binaryPath := filepath.Join(uploadDir, name)

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create upload directory")
		return
	}

	// Write binary to disk
	outFile, err := os.OpenFile(binaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create binary file")
		return
	}

	if _, err := io.Copy(outFile, file); err != nil {
		outFile.Close()
		_ = os.Remove(binaryPath)
		writeError(w, http.StatusInternalServerError, "Failed to write binary file")
		return
	}
	if err := outFile.Close(); err != nil {
		_ = os.Remove(binaryPath)
		writeError(w, http.StatusInternalServerError, "Failed to finalize binary file")
		return
	}

	// Validate pack by running manifest command
	factory, err := packrunner.NewFactory(h.dataDir)
	if err != nil {
		_ = os.Remove(binaryPath)
		writeError(w, http.StatusInternalServerError, "Failed to create pack runner factory")
		return
	}

	cfg := config.PackConfig{
		Name:   name,
		Type:   config.PackTypeUpload,
		Source: binaryPath,
	}

	if _, err := factory.GetManifest(r.Context(), cfg, nil, nil); err != nil {
		_ = os.Remove(binaryPath)
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("Invalid pack binary: manifest validation failed: %v", err))
		return
	}

	// Save to database
	pack := &db.Pack{
		Name:   name,
		Type:   string(config.PackTypeUpload),
		Source: binaryPath,
		Status: "installed",
	}

	if err := h.packStore.Upsert(r.Context(), pack, getUserEmail(r)); err != nil {
		_ = os.Remove(binaryPath)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, pack)
}
