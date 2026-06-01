package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultAppConfig(t *testing.T) {
	assert.Equal(t, AppConfig{
		Parallelism:       5,
		TerraformVersion:  "",
		PackLogsEnabled:   true,
		SSHLoggingEnabled: false,
	}, DefaultAppConfig())
}
