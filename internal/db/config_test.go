package db

import (
	"context"
	"encoding/json"
	"maps"
	"testing"

	"github.com/IBM/simrun/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAppConfig_EmptyMap(t *testing.T) {
	assert.Equal(t, config.DefaultAppConfig(), parseAppConfig(map[string]json.RawMessage{}))
}

func TestParseAppConfig_PartialKeys(t *testing.T) {
	got := parseAppConfig(map[string]json.RawMessage{
		"parallelism": json.RawMessage(`10`),
	})
	want := config.DefaultAppConfig()
	want.Parallelism = 10
	assert.Equal(t, want, got)
}

func TestParseAppConfig_InvalidJSONKeepsDefault(t *testing.T) {
	got := parseAppConfig(map[string]json.RawMessage{
		"parallelism":         json.RawMessage(`"not-a-number"`),
		"terraform_version":   json.RawMessage(`123`),
		"pack_logs_enabled":   json.RawMessage(`"yes"`),
		"ssh_logging_enabled": json.RawMessage(`{bad json`),
	})
	assert.Equal(t, config.DefaultAppConfig(), got)
}

func TestParseAppConfig_NonPositiveParallelismKeepsDefault(t *testing.T) {
	for _, v := range []string{`0`, `-1`} {
		t.Run(v, func(t *testing.T) {
			got := parseAppConfig(map[string]json.RawMessage{
				"parallelism": json.RawMessage(v),
			})
			assert.Equal(t, 5, got.Parallelism)
		})
	}
}

func TestParseAppConfig_AllSet(t *testing.T) {
	got := parseAppConfig(map[string]json.RawMessage{
		"parallelism":               json.RawMessage(`12`),
		"terraform_version":         json.RawMessage(`"1.6.0"`),
		"pack_logs_enabled":         json.RawMessage(`false`),
		"ssh_logging_enabled":       json.RawMessage(`true`),
		"run_log_retention_enabled": json.RawMessage(`false`),
		"run_log_retention_days":    json.RawMessage(`14`),
		"run_retention_enabled":     json.RawMessage(`true`),
		"run_retention_days":        json.RawMessage(`90`),
	})
	assert.Equal(t, config.AppConfig{
		Parallelism:            12,
		TerraformVersion:       "1.6.0",
		PackLogsEnabled:        false,
		SSHLoggingEnabled:      true,
		RunLogRetentionEnabled: false,
		RunLogRetentionDays:    14,
		RunRetentionEnabled:    true,
		RunRetentionDays:       90,
	}, got)
}

// The retention day fields guard against non-positive values like parallelism,
// so a stored 0/-1 cannot silently configure immediate deletion.
func TestParseAppConfig_NonPositiveRetentionDaysKeepsDefault(t *testing.T) {
	def := config.DefaultAppConfig()
	for _, v := range []string{`0`, `-1`} {
		t.Run(v, func(t *testing.T) {
			got := parseAppConfig(map[string]json.RawMessage{
				"run_log_retention_days": json.RawMessage(v),
				"run_retention_days":     json.RawMessage(v),
			})
			assert.Equal(t, def.RunLogRetentionDays, got.RunLogRetentionDays)
			assert.Equal(t, def.RunRetentionDays, got.RunRetentionDays)
		})
	}
}

func TestAppConfigKVs_MarshalsToExpectedJSON(t *testing.T) {
	c := config.AppConfig{
		Parallelism:            7,
		TerraformVersion:       "1.5.7",
		PackLogsEnabled:        true,
		SSHLoggingEnabled:      false,
		RunLogRetentionEnabled: true,
		RunLogRetentionDays:    7,
		RunRetentionEnabled:    false,
		RunRetentionDays:       30,
	}

	want := map[string]string{
		"parallelism":               `7`,
		"terraform_version":         `"1.5.7"`,
		"pack_logs_enabled":         `true`,
		"ssh_logging_enabled":       `false`,
		"run_log_retention_enabled": `true`,
		"run_log_retention_days":    `7`,
		"run_retention_enabled":     `false`,
		"run_retention_days":        `30`,
	}

	kvs := appConfigKVs(c)
	require.Len(t, kvs, len(want))
	for _, p := range kvs {
		raw, err := json.Marshal(p.val)
		require.NoError(t, err, "marshal %s", p.key)
		assert.Equal(t, want[p.key], string(raw), "key %s", p.key)
	}
}

func TestFakeConfigStore_UpdateGetAppConfigRoundtrip(t *testing.T) {
	f := &fakeConfigStore{data: map[string]json.RawMessage{}}
	ctx := context.Background()

	want := config.AppConfig{
		Parallelism:            12,
		TerraformVersion:       "1.6.0",
		PackLogsEnabled:        false,
		SSHLoggingEnabled:      true,
		RunLogRetentionEnabled: false,
		RunLogRetentionDays:    14,
		RunRetentionEnabled:    true,
		RunRetentionDays:       90,
	}
	require.NoError(t, f.UpdateAppConfig(ctx, want))

	got, err := f.GetAppConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	assert.Equal(t, []string{
		"parallelism", "terraform_version", "pack_logs_enabled", "ssh_logging_enabled",
		"run_log_retention_enabled", "run_log_retention_days",
		"run_retention_enabled", "run_retention_days",
	}, f.sets)
}

// fakeConfigStore implements ConfigStore in-memory, recording Set calls so
// UpdateAppConfig can be exercised without a real Postgres connection.
type fakeConfigStore struct {
	data map[string]json.RawMessage
	sets []string // ordered list of keys passed to Set
}

func (f *fakeConfigStore) Get(_ context.Context, key string) (json.RawMessage, error) {
	return f.data[key], nil
}

func (f *fakeConfigStore) Set(_ context.Context, key string, value json.RawMessage) error {
	f.data[key] = value
	f.sets = append(f.sets, key)
	return nil
}

func (f *fakeConfigStore) GetAll(_ context.Context) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(f.data))
	maps.Copy(out, f.data)
	return out, nil
}

func (f *fakeConfigStore) GetAppConfig(ctx context.Context) (config.AppConfig, error) {
	all, err := f.GetAll(ctx)
	if err != nil {
		return config.DefaultAppConfig(), err
	}
	return parseAppConfig(all), nil
}

func (f *fakeConfigStore) UpdateAppConfig(ctx context.Context, c config.AppConfig) error {
	for _, p := range appConfigKVs(c) {
		raw, err := json.Marshal(p.val)
		if err != nil {
			return err
		}
		if err := f.Set(ctx, p.key, raw); err != nil {
			return err
		}
	}
	return nil
}

var _ ConfigStore = (*fakeConfigStore)(nil)
