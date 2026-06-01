package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearSREnv unsets every SR_* env var visible in the current process so each
// test starts from a clean slate. Discovered at runtime so future SR_* keys
// are handled automatically.
func clearSREnv(t *testing.T) {
	t.Helper()
	for _, e := range os.Environ() {
		if k, _, ok := strings.Cut(e, "="); ok && strings.HasPrefix(k, "SR_") {
			t.Setenv(k, "")
		}
	}
}

func TestLoadBootstrap_MissingDatabaseURL(t *testing.T) {
	clearSREnv(t)

	b, err := LoadBootstrap()
	require.Error(t, err)
	assert.Nil(t, b)
}

func TestLoadBootstrap_Defaults(t *testing.T) {
	clearSREnv(t)

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("SR_DATABASE_URL", "postgres://example/db")

	b, err := LoadBootstrap()
	require.NoError(t, err)

	wantDataDir := filepath.Join(fakeHome, ".simrun")
	assert.Equal(t, &Bootstrap{
		DatabaseURL:   "postgres://example/db",
		WebPort:       "8080",
		DataDir:       wantDataDir,
		EncryptionKey: filepath.Join(wantDataDir, "encryption.key"),
		Auth: AuthBootstrap{
			SessionTTL: 168 * time.Hour,
		},
	}, b)
}

func TestLoadBootstrap_AllOverrides(t *testing.T) {
	clearSREnv(t)

	t.Setenv("SR_DATABASE_URL", "postgres://override/db")
	t.Setenv("SR_DATA_DIR", "/tmp/simrun-data")
	t.Setenv("SR_WEB_PORT", "9090")
	t.Setenv("SR_ENCRYPTION_KEY_FILE", "/etc/simrun/key")
	t.Setenv("SR_DEBUG", "1")
	t.Setenv("SR_WEB_DEV", "1")
	t.Setenv("SR_WEB_URL", "https://simrun.example.com")
	t.Setenv("SR_GOOGLE_CLIENT_ID", "client-id-123")
	t.Setenv("SR_GOOGLE_CLIENT_SECRET", "client-secret-456")
	t.Setenv("SR_GOOGLE_ALLOWED_DOMAIN", "example.com")
	t.Setenv("SR_AUTH_SESSION_TTL_HOURS", "24")

	b, err := LoadBootstrap()
	require.NoError(t, err)

	assert.Equal(t, &Bootstrap{
		DatabaseURL:   "postgres://override/db",
		WebPort:       "9090",
		DataDir:       "/tmp/simrun-data",
		Debug:         true,
		EncryptionKey: "/etc/simrun/key",
		DevMode:       true,
		WebURL:        "https://simrun.example.com",
		Auth: AuthBootstrap{
			GoogleClientID:     "client-id-123",
			GoogleClientSecret: "client-secret-456",
			AllowedDomain:      "example.com",
			SessionTTL:         24 * time.Hour,
		},
	}, b)
}

func TestLoadBootstrap_DebugFalsyValues(t *testing.T) {
	cases := []struct {
		name string
		val  string
		want bool
	}{
		{"unset", "", false},
		{"zero", "0", false},
		{"false", "false", false},
		{"one", "1", true},
		{"true", "true", true},
		{"anything", "yes", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearSREnv(t)
			t.Setenv("SR_DATABASE_URL", "postgres://example/db")
			if tc.val != "" {
				t.Setenv("SR_DEBUG", tc.val)
			}

			b, err := LoadBootstrap()
			require.NoError(t, err)
			assert.Equal(t, tc.want, b.Debug, "SR_DEBUG=%q", tc.val)
		})
	}
}

func TestLoadBootstrap_InvalidSessionTTL(t *testing.T) {
	cases := []string{"abc", "0", "-5", "1.5"}

	for _, val := range cases {
		t.Run(val, func(t *testing.T) {
			clearSREnv(t)
			t.Setenv("SR_DATABASE_URL", "postgres://example/db")
			t.Setenv("SR_AUTH_SESSION_TTL_HOURS", val)

			b, err := LoadBootstrap()
			require.NoError(t, err)
			assert.Equal(t, 168*time.Hour, b.Auth.SessionTTL)
		})
	}
}
