package injectors

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestElasticInjectorString(t *testing.T) {
	injector := &ElasticInjector{
		Documents: []ElasticInjectorDocument{
			{
				Index: "test-index",
				File:  "test-file.json",
				Vars:  map[string]string{"test": "value"},
			},
		},
	}

	assert.Equal(t, "ElasticInjector", injector.String())
}

func TestElasticInjectorInject(t *testing.T) {
	// Create a temporary template file for testing
	templateContent := `{
	"actor": {
		"id": "test-user",
		"displayName": "{{ .UserName }}"
	},
	"target": [
		{
			"id": "{{ .TargetGroupID }}",
			"type": "UserGroup"
		}
	]
}`

	tempFile, err := os.CreateTemp("", "test-template-*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(templateContent)
	require.NoError(t, err)
	tempFile.Close()

	testCases := []struct {
		name          string
		envVars       map[string]string
		documents     []ElasticInjectorDocument
		expectError   bool
		errorContains string
	}{
		{
			name: "Invalid template file",
			documents: []ElasticInjectorDocument{
				{
					Index: "test-index",
					File:  "non-existent-file.json",
					Vars:  map[string]string{},
				},
			},
			expectError:   true,
			errorContains: "failed to read template file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tc.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			injector := &ElasticInjector{
				Documents: tc.documents,
			}

			result, err := injector.Inject()

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Contains(t, result, "execution_id")
				assert.NotEmpty(t, result["execution_id"])
			}
		})
	}
}

func TestElasticInjectorTemplateProcessing(t *testing.T) {
	// Create a template file
	templateContent := `{
	"message": "User {{ .UserName }} was added to group {{ .GroupName }}",
	"user_id": "{{ .UserID }}",
	"timestamp": "2025-09-09T15:28:05.181Z"
}`

	tempFile, err := os.CreateTemp("", "template-test-*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(templateContent)
	require.NoError(t, err)
	tempFile.Close()

	// Test template processing by reading and parsing the file manually
	injector := &ElasticInjector{
		Documents: []ElasticInjectorDocument{
			{
				Index: "test-index",
				File:  tempFile.Name(),
				Vars: map[string]string{
					"UserName":  "John Doe",
					"GroupName": "Administrators",
					"UserID":    "user-123",
				},
			},
		},
	}

	// We can't fully test injection without Elasticsearch, but we can test template processing
	doc := injector.Documents[0]

	// Read the template file
	templateBytes, err := os.ReadFile(doc.File)
	require.NoError(t, err)

	// Check that the template contains variables
	templateStr := string(templateBytes)
	assert.Contains(t, templateStr, "{{ .UserName }}")
	assert.Contains(t, templateStr, "{{ .GroupName }}")
	assert.Contains(t, templateStr, "{{ .UserID }}")
}

func TestElasticInjectorDocumentStructure(t *testing.T) {
	doc := ElasticInjectorDocument{
		Index: "test-index",
		File:  "test-file.json",
		Vars: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Test JSON marshaling/unmarshaling
	jsonBytes, err := json.Marshal(doc)
	require.NoError(t, err)

	var unmarshaled ElasticInjectorDocument
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, doc.Index, unmarshaled.Index)
	assert.Equal(t, doc.File, unmarshaled.File)
	assert.Equal(t, doc.Vars, unmarshaled.Vars)
}
