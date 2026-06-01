package web_test

import (
	"net/http"
	"testing"

	"github.com/IBM/simrun/internal/testutil/testserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleHealth(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/health")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, testserver.ReadBody(t, resp), `"status":"ok"`)
}

func TestHandleAuthMe_AuthDisabled_ReturnsAnonymous(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/auth/me")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := testserver.ReadBody(t, resp)
	assert.Contains(t, body, `"email":"anonymous"`)
	assert.Contains(t, body, `"name":"Anonymous"`)
}
