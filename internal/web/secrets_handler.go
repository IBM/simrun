package web

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/IBM/simrun/internal/crypto"
	"github.com/IBM/simrun/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// SecretHandlers provides REST handlers for secret management.
type SecretHandlers struct {
	secretStore db.SecretStore
	encryptor   *crypto.Encryptor
}

// NewSecretHandlers creates a new SecretHandlers instance.
func NewSecretHandlers(secretStore db.SecretStore, encryptor *crypto.Encryptor) *SecretHandlers {
	return &SecretHandlers{
		secretStore: secretStore,
		encryptor:   encryptor,
	}
}

func (h *SecretHandlers) requireEncryptor(w http.ResponseWriter) bool {
	if h.encryptor == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets feature is disabled (SR_ENCRYPTION_KEY not configured)")
		return false
	}
	return true
}

// HandleListSecrets handles GET /api/secrets
func (h *SecretHandlers) HandleListSecrets(w http.ResponseWriter, r *http.Request) {
	groups, err := h.secretStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]SecretGroupResponse, len(groups))
	for i, g := range groups {
		resp[i] = h.toResponse(g)
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleGetSecret handles GET /api/secrets/{id}
func (h *SecretHandlers) HandleGetSecret(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid secret ID")
		return
	}

	group, err := h.secretStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "secret group not found")
		return
	}

	writeJSON(w, http.StatusOK, h.toResponse(*group))
}

// HandleSaveSecret handles POST /api/secrets
func (h *SecretHandlers) HandleSaveSecret(w http.ResponseWriter, r *http.Request) {
	if !h.requireEncryptor(w) {
		return
	}
	var req CreateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	encrypted, err := h.encryptEntries(req.Entries, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encryption failed")
		return
	}

	entriesJSON, err := json.Marshal(encrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal entries")
		return
	}

	group, err := h.secretStore.Save(r.Context(), req.Name, req.Description, entriesJSON, getUserEmail(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, h.toResponse(*group))
}

// HandleUpdateSecret handles PUT /api/secrets/{id}
func (h *SecretHandlers) HandleUpdateSecret(w http.ResponseWriter, r *http.Request) {
	if !h.requireEncryptor(w) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid secret ID")
		return
	}

	var req UpdateSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Load existing to preserve unchanged values
	existing, err := h.secretStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "secret group not found")
		return
	}

	var existingEntries map[string]string
	if err := json.Unmarshal(existing.Entries, &existingEntries); err != nil {
		existingEntries = make(map[string]string)
	}

	encrypted, err := h.encryptEntries(req.Entries, existingEntries)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encryption failed")
		return
	}

	entriesJSON, err := json.Marshal(encrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal entries")
		return
	}

	if err := h.secretStore.Update(r.Context(), id, req.Name, req.Description, entriesJSON, getUserEmail(r)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteSecret handles DELETE /api/secrets/{id}
func (h *SecretHandlers) HandleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid secret ID")
		return
	}

	if err := h.secretStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// encryptEntries encrypts entry values. If value is nil, preserves the existing
// encrypted value from existingEncrypted. Only keys present in entries are kept.
func (h *SecretHandlers) encryptEntries(entries []SecretEntryRequest, existingEncrypted map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	for _, entry := range entries {
		if entry.Key == "" {
			continue
		}
		if entry.Value != nil {
			enc, err := h.encryptor.Encrypt(*entry.Value)
			if err != nil {
				return nil, err
			}
			result[entry.Key] = enc
		} else if existingEncrypted != nil {
			if existing, ok := existingEncrypted[entry.Key]; ok {
				result[entry.Key] = existing
			}
		}
	}
	return result, nil
}

// toResponse converts a SecretGroup to a response with only key names (no values).
func (h *SecretHandlers) toResponse(g db.SecretGroup) SecretGroupResponse {
	var entries map[string]string
	if err := json.Unmarshal(g.Entries, &entries); err != nil {
		entries = make(map[string]string)
	}

	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return SecretGroupResponse{
		ID:          g.ID.String(),
		Name:        g.Name,
		Description: g.Description,
		Keys:        keys,
		CreatedBy:   g.CreatedBy,
		UpdatedBy:   g.UpdatedBy,
		CreatedAt:   g.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   g.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
