package web_test

import (
	"net/http"
	"testing"

	"github.com/IBM/simrun/internal/testutil/testserver"
	"github.com/IBM/simrun/internal/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Retention day fields are floored at 1 so a config write cannot configure
// immediate deletion of run logs or assessments.
func TestHandleUpdateConfig_RetentionDaysRejectsZero(t *testing.T) {
	for _, key := range []string{"run_log_retention_days", "run_retention_days"} {
		t.Run(key, func(t *testing.T) {
			ts := testserver.New(t)

			before, err := ts.Stores.Config.GetAppConfig(t.Context())
			require.NoError(t, err)

			resp := ts.Put(t, "/api/config", web.UpdateConfigRequest{
				Key:   key,
				Value: []byte(`0`),
			})
			defer resp.Body.Close()
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)

			// The rejected write must leave the stored config untouched.
			after, err := ts.Stores.Config.GetAppConfig(t.Context())
			require.NoError(t, err)
			assert.Equal(t, before, after)
		})
	}
}

func TestHandleUpdateConfig_RetentionDaysPersistsValid(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Put(t, "/api/config", web.UpdateConfigRequest{
		Key:   "run_retention_days",
		Value: []byte(`14`),
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	cfg, err := ts.Stores.Config.GetAppConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 14, cfg.RunRetentionDays)
}
