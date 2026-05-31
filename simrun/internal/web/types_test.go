package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSHConnectorConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     SSHConnectorConfig
		wantErr string
	}{
		{"valid no port", SSHConnectorConfig{Host: "h", Username: "u"}, ""},
		{"valid with port", SSHConnectorConfig{Host: "h", Username: "u", Port: 22}, ""},
		{"missing host", SSHConnectorConfig{Username: "u"}, "host is required"},
		{"missing username", SSHConnectorConfig{Host: "h"}, "username is required"},
		{"port negative", SSHConnectorConfig{Host: "h", Username: "u", Port: -1}, "port must be 0-65535"},
		{"port too large", SSHConnectorConfig{Host: "h", Username: "u", Port: 70000}, "port must be 0-65535"},
		{"port at upper bound", SSHConnectorConfig{Host: "h", Username: "u", Port: 65535}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
