// Package testserver provides a one-line setup for HTTP-handler tests.
// New(t) returns an httptest.Server backed by in-memory fakes. Tests inspect
// the embedded Stores to verify side effects.
package testserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/IBM/simrun/simrun/internal/credentials"
	"github.com/IBM/simrun/simrun/internal/crypto"
	"github.com/IBM/simrun/simrun/internal/testutil/fakes"
	"github.com/IBM/simrun/simrun/internal/web"
	"github.com/IBM/simrun/simrun/internal/web/auth"
	"github.com/stretchr/testify/require"
)

// TS bundles a running httptest.Server with the in-memory fakes that back it.
type TS struct {
	*httptest.Server
	Stores    *fakes.Stores
	Encryptor *crypto.Encryptor
	DataDir   string
}

// New returns a fully-wired test server backed by in-memory fakes.
// Auth is disabled (empty Google client ID/secret).
func New(t *testing.T) *TS {
	t.Helper()
	stores := fakes.New()

	dataDir := t.TempDir()
	keyPath := filepath.Join(dataDir, "encryption.key")
	encryptor, err := crypto.LoadOrGenerateKey(keyPath)
	require.NoError(t, err, "init encryptor")

	hub := web.NewHub()
	go hub.Run()

	runLogRegistry := web.NewRunLogRegistry()

	credResolver := credentials.NewResolver(stores.Connector, stores.Secret, encryptor)
	exporter := web.NewResultExporter(stores.Connector, credResolver)
	scenarioService := web.NewScenarioService(
		stores.Run, stores.Scenario, stores.Pack, stores.Config,
		credResolver, exporter, hub, runLogRegistry, dataDir,
	)
	packHandlers := web.NewPackHandlers(stores.Pack, dataDir)
	secretHandlers := web.NewSecretHandlers(stores.Secret, encryptor)
	connectorHandlers := web.NewConnectorHandlers(stores.Connector, stores.Secret, stores.Scenario, stores.Run, credResolver)
	authHandlers := auth.NewHandlers(stores.Session, auth.Config{})
	scheduler := web.NewScheduler(stores.Schedule, stores.Scenario, scenarioService)
	scheduleHandlers := web.NewScheduleHandlers(stores.Schedule, stores.Scenario, scheduler)
	handlers := web.NewHandlers(scenarioService, stores.Scenario, stores.Run, stores.Config, scheduler, dataDir)

	srv := web.NewServer(
		handlers, packHandlers, secretHandlers, scheduleHandlers,
		connectorHandlers, authHandlers, hub,
		&web.ServerConfig{Port: "0", DevMode: false},
		stores.Session,
	)

	httpSrv := httptest.NewServer(srv.Router())
	t.Cleanup(httpSrv.Close)

	return &TS{
		Server:    httpSrv,
		Stores:    stores,
		Encryptor: encryptor,
		DataDir:   dataDir,
	}
}

// Get issues a GET against the test server.
func (ts *TS) Get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	require.NoError(t, err)
	return resp
}

// Post issues a POST with a JSON body.
func (ts *TS) Post(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	return ts.do(t, http.MethodPost, path, body)
}

// Put issues a PUT with a JSON body.
func (ts *TS) Put(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	return ts.do(t, http.MethodPut, path, body)
}

// Delete issues a DELETE.
func (ts *TS) Delete(t *testing.T, path string) *http.Response {
	t.Helper()
	return ts.do(t, http.MethodDelete, path, nil)
}

func (ts *TS) do(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			reader = bytes.NewBufferString(v)
		case []byte:
			reader = bytes.NewBuffer(v)
		default:
			b, err := json.Marshal(body)
			require.NoError(t, err)
			reader = bytes.NewBuffer(b)
		}
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// ReadBody reads the response body as a string.
func ReadBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

// DecodeJSON reads the response body and unmarshals into out.
func DecodeJSON(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
}
