package testserver_test

import (
	"net/http"
	"testing"

	"github.com/IBM/simrun/internal/testutil/testserver"
	"github.com/stretchr/testify/assert"
)

func TestTestServer_Health(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/health")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
