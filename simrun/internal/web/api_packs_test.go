package web_test

import (
	"net/http"
	"testing"

	"github.com/IBM/simrun/simrun/internal/db"
	"github.com/IBM/simrun/simrun/internal/testutil/testserver"
	"github.com/IBM/simrun/simrun/internal/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleListPacks_Empty(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/packs")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var packs []db.Pack
	testserver.DecodeJSON(t, resp, &packs)
	assert.Empty(t, packs)
}

func TestHandleInstallPack_AndList(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/packs/install", web.InstallPackRequest{
		Name:    "base",
		Type:    "remote",
		Source:  "owner/repo",
		Version: "v1.0.0",
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = ts.Get(t, "/api/packs")
	defer resp.Body.Close()
	var packs []db.Pack
	testserver.DecodeJSON(t, resp, &packs)
	require.Len(t, packs, 1)
	assert.Equal(t, "base", packs[0].Name)
	assert.Equal(t, "installed", packs[0].Status)
}

func TestHandleInstallPack_RejectsGoRemote(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/packs/install", web.InstallPackRequest{
		Name:    "legacy",
		Type:    "go-remote",
		Source:  "github.com/owner/repo",
		Version: "v1.0.0",
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp = ts.Get(t, "/api/packs")
	defer resp.Body.Close()
	var packs []db.Pack
	testserver.DecodeJSON(t, resp, &packs)
	assert.Empty(t, packs, "go-remote install must not persist a pack row")
}

func TestHandleDeletePack(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	require.NoError(t, ts.Stores.Pack.Upsert(ctx, &db.Pack{
		Name: "to-delete", Type: "remote", Source: "x/y", Status: "installed",
	}, "tester"))

	resp := ts.Delete(t, "/api/packs/to-delete")
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	_, err := ts.Stores.Pack.Get(ctx, "to-delete")
	assert.Error(t, err)
}

func TestHandleGetPackParameters(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	params := map[string]any{"region": "us-east-1", "instance_count": float64(3)}
	require.NoError(t, ts.Stores.Pack.Upsert(ctx, &db.Pack{
		Name: "pack1", Type: "remote", Source: "x/y", Status: "installed",
		Parameters: params,
	}, "tester"))

	resp := ts.Get(t, "/api/packs/pack1/parameters")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got map[string]any
	testserver.DecodeJSON(t, resp, &got)
	assert.Equal(t, params, got["parameters"])
}

func TestHandleGetPackParameters_NotFound(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/packs/nope/parameters")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleUpdatePackParameters(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	require.NoError(t, ts.Stores.Pack.Upsert(ctx, &db.Pack{
		Name: "pack1", Type: "remote", Source: "x/y", Status: "installed",
	}, "tester"))

	newParams := map[string]any{"region": "eu-west-1"}
	resp := ts.Put(t, "/api/packs/pack1/parameters", web.UpdatePackParametersRequest{Parameters: newParams})
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Permissive fallback: manifest fetch fails for the unresolvable remote
	// source, so every key is reported as unknown but accepted.
	var body map[string]any
	testserver.DecodeJSON(t, resp, &body)
	assert.Equal(t, newParams, body["parameters"])
	unknown, ok := body["unknown_keys"].([]any)
	require.True(t, ok, "expected unknown_keys in response, got %T", body["unknown_keys"])
	assert.ElementsMatch(t, []any{"region"}, unknown)

	got, err := ts.Stores.Pack.Get(ctx, "pack1")
	require.NoError(t, err)
	assert.Equal(t, newParams, got.Parameters)
}
