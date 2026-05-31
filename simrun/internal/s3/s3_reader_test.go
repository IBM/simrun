package s3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseS3URL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expected    *S3ObjectInfo
		expectError bool
	}{
		{
			name: "valid virtual-hosted-style URL",
			url:  "https://my-bucket.s3.us-west-2.amazonaws.com/path/to/file.yaml",
			expected: &S3ObjectInfo{
				Bucket: "my-bucket",
				Key:    "path/to/file.yaml",
				Region: "us-west-2",
			},
			expectError: false,
		},
		{
			name: "valid virtual-hosted-style URL with nested path",
			url:  "https://simrun-scenarios.s3.us-east-1.amazonaws.com/production/aws/scenarios.simrun.yaml",
			expected: &S3ObjectInfo{
				Bucket: "simrun-scenarios",
				Key:    "production/aws/scenarios.simrun.yaml",
				Region: "us-east-1",
			},
			expectError: false,
		},
		{
			name: "valid path-style URL",
			url:  "https://s3.us-west-2.amazonaws.com/my-bucket/scenarios/test.yaml",
			expected: &S3ObjectInfo{
				Bucket: "my-bucket",
				Key:    "scenarios/test.yaml",
				Region: "us-west-2",
			},
			expectError: false,
		},
		{
			name: "valid virtual-hosted-style URL without region (legacy format)",
			url:  "https://my-bucket.s3.amazonaws.com/file.yaml",
			expected: &S3ObjectInfo{
				Bucket: "my-bucket",
				Key:    "file.yaml",
				Region: "",
			},
			expectError: false,
		},
		{
			name:        "invalid URL - not S3",
			url:         "https://example.com/file.yaml",
			expectError: true,
		},
		{
			name:        "invalid URL - missing object key",
			url:         "https://my-bucket.s3.us-west-2.amazonaws.com/",
			expectError: true,
		},
		{
			name:        "invalid URL - not HTTPS",
			url:         "http://my-bucket.s3.us-west-2.amazonaws.com/file.yaml",
			expectError: true,
		},
		{
			name:        "invalid URL - malformed",
			url:         "not-a-url",
			expectError: true,
		},
		{
			name:        "empty URL",
			url:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseS3URL(tt.url)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Bucket, result.Bucket)
				assert.Equal(t, tt.expected.Key, result.Key)
				assert.Equal(t, tt.expected.Region, result.Region)
			}
		})
	}
}

func TestIsS3URL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid S3 virtual-hosted-style URL",
			input:    "https://my-bucket.s3.us-west-2.amazonaws.com/file.yaml",
			expected: true,
		},
		{
			name:     "valid S3 path-style URL",
			input:    "https://s3.us-west-2.amazonaws.com/my-bucket/file.yaml",
			expected: true,
		},
		{
			name:     "valid S3 URL without region",
			input:    "https://my-bucket.s3.amazonaws.com/file.yaml",
			expected: true,
		},
		{
			name:     "not an S3 URL",
			input:    "/path/to/local/file.yaml",
			expected: false,
		},
		{
			name:     "different domain",
			input:    "https://example.com/file.yaml",
			expected: false,
		},
		{
			name:     "not HTTPS",
			input:    "http://my-bucket.s3.us-west-2.amazonaws.com/file.yaml",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsS3URL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
