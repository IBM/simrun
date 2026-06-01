package web_test

import (
	"net/http"
	"testing"

	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/testutil/testserver"
	"github.com/IBM/simrun/internal/web"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCreateConnector_Elastic(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/connectors", web.CreateConnectorRequest{
		Name: "prod-elastic", Type: "elastic",
		Config: map[string]any{"kibana_url": "https://kibana.example.com"},
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Len(t, ts.Stores.Connector.All(), 1)
}

func TestHandleCreateConnector_AWS(t *testing.T) {
	ts := testserver.New(t)
	resp := ts.Post(t, "/api/connectors", web.CreateConnectorRequest{
		Name: "prod-aws", Type: "aws",
		Config: map[string]any{"role_arn": "arn:aws:iam::123:role/x"},
	})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHandleCreateConnector_GCP(t *testing.T) {
	ts := testserver.New(t)
	resp := ts.Post(t, "/api/connectors", web.CreateConnectorRequest{
		Name: "prod-gcp", Type: "gcp",
		Config: map[string]any{"project_id": "my-project"},
	})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHandleCreateConnector_Azure(t *testing.T) {
	ts := testserver.New(t)
	resp := ts.Post(t, "/api/connectors", web.CreateConnectorRequest{
		Name: "prod-azure", Type: "azure",
		Config: map[string]any{"tenant_id": "t", "subscription_id": "s"},
	})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHandleCreateConnector_Kubernetes(t *testing.T) {
	ts := testserver.New(t)
	resp := ts.Post(t, "/api/connectors", web.CreateConnectorRequest{
		Name: "prod-k8s", Type: "kubernetes",
		Config: map[string]any{
			"cluster_name":    "prod-cluster",
			"region":          "us-east-1",
			"cloud_connector": "prod-aws",
		},
	})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHandleCreateConnector_SSH(t *testing.T) {
	ts := testserver.New(t)
	resp := ts.Post(t, "/api/connectors", web.CreateConnectorRequest{
		Name: "prod-ssh", Type: "ssh",
		Config: map[string]any{"host": "host.example.com", "username": "u"},
	})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestHandleCreateConnector_RejectsMissing(t *testing.T) {
	cases := []struct {
		name    string
		req     web.CreateConnectorRequest
		wantErr string
	}{
		{
			name:    "missing name",
			req:     web.CreateConnectorRequest{Type: "elastic", Config: map[string]any{"kibana_url": "https://k"}},
			wantErr: "name and type are required",
		},
		{
			name:    "missing type",
			req:     web.CreateConnectorRequest{Name: "x", Config: map[string]any{}},
			wantErr: "name and type are required",
		},
		{
			name:    "elastic missing kibana_url",
			req:     web.CreateConnectorRequest{Name: "x", Type: "elastic", Config: map[string]any{}},
			wantErr: "kibana_url is required",
		},
		{
			name:    "kubernetes missing cluster",
			req:     web.CreateConnectorRequest{Name: "x", Type: "kubernetes", Config: map[string]any{"region": "us-east-1"}},
			wantErr: "cluster_name, region, and cloud_connector",
		},
		{
			name:    "ssh missing host",
			req:     web.CreateConnectorRequest{Name: "x", Type: "ssh", Config: map[string]any{"username": "u"}},
			wantErr: "host is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := testserver.New(t)
			resp := ts.Post(t, "/api/connectors", tc.req)
			defer resp.Body.Close()
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Contains(t, testserver.ReadBody(t, resp), tc.wantErr)
		})
	}
}

func TestHandleListConnectors(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	for _, name := range []string{"a-conn", "b-conn"} {
		_, err := ts.Stores.Connector.Save(ctx, name, "elastic", "", nil, []byte(`{"kibana_url":"x"}`), false, "tester")
		require.NoError(t, err)
	}

	resp := ts.Get(t, "/api/connectors")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var list []db.Connector
	testserver.DecodeJSON(t, resp, &list)
	assert.Len(t, list, 2)
}

func TestHandleGetConnector_NotFound(t *testing.T) {
	ts := testserver.New(t)
	resp := ts.Get(t, "/api/connectors/"+uuid.New().String())
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleUpdateConnector(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	c, err := ts.Stores.Connector.Save(ctx, "c1", "elastic", "", nil, []byte(`{"kibana_url":"https://old"}`), false, "tester")
	require.NoError(t, err)

	resp := ts.Put(t, "/api/connectors/"+c.ID.String(), web.UpdateConnectorRequest{
		Name:    "c1-renamed",
		Config:  map[string]any{"kibana_url": "https://new"},
		Enabled: true,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	updated, err := ts.Stores.Connector.Get(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "c1-renamed", updated.Name)
	assert.Contains(t, string(updated.Config), "https://new")
}

func TestHandleDeleteConnector(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	c, err := ts.Stores.Connector.Save(ctx, "to-delete", "elastic", "", nil, []byte(`{"kibana_url":"x"}`), false, "tester")
	require.NoError(t, err)

	resp := ts.Delete(t, "/api/connectors/"+c.ID.String())
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	_, err = ts.Stores.Connector.Get(ctx, c.ID)
	assert.Error(t, err)
}
