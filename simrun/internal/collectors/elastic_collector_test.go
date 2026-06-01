package collectors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadElasticCollectorConfig_Empty(t *testing.T) {
	// UserAgentField defaults to the ECS user_agent.original field. Automatic
	// user-agent correlation (the execution UUID injected into the UA header) is
	// the primary way the collector finds detonation-related logs; the standard
	// collect config specifies only an index and relies on this default. Without
	// it, buildSearchQuery emits no user-agent clause and collection returns
	// nothing for scenarios that don't set additionalFields.
	cfg := LoadElasticCollectorConfig(map[string]string{})
	assert.Equal(t, &ElasticCollectorConfig{
		OutputDir:      "./logs",
		UserAgentField: "user_agent.original",
	}, cfg)
}

func TestLoadElasticCollectorConfig_AllSet(t *testing.T) {
	env := map[string]string{
		"SR_ELASTIC_URL":                "https://es.example.com",
		"SR_ELASTIC_CLOUD_ID":           "deployment:abc",
		"SR_ELASTIC_API_KEY":            "k",
		"SR_COLLECTOR_OUTPUT_DIR":       "/var/logs",
		"SR_COLLECTOR_USER_AGENT_FIELD": "user_agent.original",
	}
	cfg := LoadElasticCollectorConfig(env)
	assert.Equal(t, &ElasticCollectorConfig{
		ElasticsearchURL: "https://es.example.com",
		CloudID:          "deployment:abc",
		APIKey:           "k",
		OutputDir:        "/var/logs",
		UserAgentField:   "user_agent.original",
	}, cfg)
}
