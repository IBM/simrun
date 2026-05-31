package fileutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvironmentS3URL(t *testing.T) {
	envVar := "TEST_S3_URL"
	expectedURL := "https://test-bucket.s3.us-west-2.amazonaws.com/test-file.yaml"

	// Test when environment variable is set
	os.Setenv(envVar, expectedURL)
	defer os.Unsetenv(envVar)

	result := GetEnvironmentS3URL(envVar)
	assert.Equal(t, expectedURL, result)

	// Test when environment variable is not set
	os.Unsetenv(envVar)
	result = GetEnvironmentS3URL(envVar)
	assert.Equal(t, "", result)
}

func TestResolveInputSource(t *testing.T) {
	envVar := "ASP_CONFIG_FROM_S3"

	tests := []struct {
		name           string
		cmdLineInputs  []string
		envVarValue    string
		expectedInputs []string
		expectError    bool
	}{
		{
			name:           "command line inputs provided",
			cmdLineInputs:  []string{"file1.yaml", "file2.yaml"},
			envVarValue:    "",
			expectedInputs: []string{"file1.yaml", "file2.yaml"},
			expectError:    false,
		},
		{
			name:           "valid S3 URL in environment variable",
			cmdLineInputs:  []string{},
			envVarValue:    "https://my-bucket.s3.us-west-2.amazonaws.com/scenarios.yaml",
			expectedInputs: []string{"https://my-bucket.s3.us-west-2.amazonaws.com/scenarios.yaml"},
			expectError:    false,
		},
		{
			name:           "command line takes precedence over environment variable",
			cmdLineInputs:  []string{"local-file.yaml"},
			envVarValue:    "https://my-bucket.s3.us-west-2.amazonaws.com/scenarios.yaml",
			expectedInputs: []string{"local-file.yaml"},
			expectError:    false,
		},
		{
			name:          "invalid S3 URL in environment variable",
			cmdLineInputs: []string{},
			envVarValue:   "not-a-valid-url",
			expectError:   true,
		},
		{
			name:          "no input source provided",
			cmdLineInputs: []string{},
			envVarValue:   "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variable
			if tt.envVarValue != "" {
				os.Setenv(envVar, tt.envVarValue)
				defer os.Unsetenv(envVar)
			} else {
				os.Unsetenv(envVar)
			}

			result, err := ResolveInputSource(tt.cmdLineInputs, envVar)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedInputs, result)
			}
		})
	}
}
