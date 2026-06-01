package web_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/IBM/simrun/internal/testutil/testserver"
	"github.com/IBM/simrun/internal/web"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestSecretCRUD(t *testing.T) {
	ts := testserver.New(t)

	// Create
	resp := ts.Post(t, "/api/secrets", web.CreateSecretRequest{
		Name:        "elastic-prod",
		Description: "production elastic credentials",
		Entries: []web.SecretEntryRequest{
			{Key: "SR_ELASTIC_API_KEY", Value: ptr("super-secret")},
			{Key: "SR_ELASTIC_URL", Value: ptr("https://es.example.com")},
		},
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var saved web.SecretGroupResponse
	testserver.DecodeJSON(t, resp, &saved)
	assert.Equal(t, "elastic-prod", saved.Name)
	assert.ElementsMatch(t, []string{"SR_ELASTIC_API_KEY", "SR_ELASTIC_URL"}, saved.Keys)
	id := uuid.MustParse(saved.ID)

	// List → values redacted to keys only
	resp = ts.Get(t, "/api/secrets")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list []web.SecretGroupResponse
	testserver.DecodeJSON(t, resp, &list)
	require.Len(t, list, 1)

	// Get → still no values, only keys
	resp = ts.Get(t, "/api/secrets/"+id.String())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got web.SecretGroupResponse
	testserver.DecodeJSON(t, resp, &got)
	assert.ElementsMatch(t, []string{"SR_ELASTIC_API_KEY", "SR_ELASTIC_URL"}, got.Keys)

	// Verify the underlying store holds encrypted (not plaintext) values.
	stored, err := ts.Stores.Secret.Get(t.Context(), id)
	require.NoError(t, err)
	var storedEntries map[string]string
	require.NoError(t, json.Unmarshal(stored.Entries, &storedEntries))
	assert.NotEqual(t, "super-secret", storedEntries["SR_ELASTIC_API_KEY"], "value should be encrypted in store")
	decrypted, err := ts.Encryptor.Decrypt(storedEntries["SR_ELASTIC_API_KEY"])
	require.NoError(t, err)
	assert.Equal(t, "super-secret", decrypted)

	// Update — change description, rotate one secret, keep the other
	resp = ts.Put(t, "/api/secrets/"+id.String(), web.UpdateSecretRequest{
		Name:        "elastic-prod",
		Description: "updated",
		Entries: []web.SecretEntryRequest{
			{Key: "SR_ELASTIC_API_KEY", Value: ptr("rotated")},
			{Key: "SR_ELASTIC_URL", Value: nil}, // keep existing
		},
	})
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	stored, err = ts.Stores.Secret.Get(t.Context(), id)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(stored.Entries, &storedEntries))
	dec, err := ts.Encryptor.Decrypt(storedEntries["SR_ELASTIC_API_KEY"])
	require.NoError(t, err)
	assert.Equal(t, "rotated", dec)
	dec, err = ts.Encryptor.Decrypt(storedEntries["SR_ELASTIC_URL"])
	require.NoError(t, err)
	assert.Equal(t, "https://es.example.com", dec, "URL should be preserved unchanged")

	// Delete
	resp = ts.Delete(t, "/api/secrets/"+id.String())
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	resp = ts.Get(t, "/api/secrets/"+id.String())
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestHandleSaveSecret_RejectsEmptyName(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/secrets", web.CreateSecretRequest{Name: ""})
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, testserver.ReadBody(t, resp), "name is required")
}

func TestHandleGetSecret_BadID(t *testing.T) {
	ts := testserver.New(t)
	resp := ts.Get(t, "/api/secrets/not-a-uuid")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
