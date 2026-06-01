package detonators

import (
	"testing"

	"github.com/IBM/simrun/internal/config"
)

func TestTerraformEnvVars_PerSimOverridesPackLevel(t *testing.T) {
	d := &SimrunDetonator{
		packConfig: config.PackConfig{
			Parameters: map[string]any{
				"aws_region":      "us-east-1",
				"resource_prefix": "asp-pack",
				"legacy_only":     "kept",
			},
		},
		params: map[string]any{
			"aws_region":   "us-west-2",
			"per_sim_only": "scenario",
		},
	}

	env := d.terraformEnvVars("exec-1")

	cases := map[string]string{
		"TF_VAR_aws_region":      "us-west-2", // per-sim wins
		"TF_VAR_resource_prefix": "asp-pack",  // pack-level only
		"TF_VAR_legacy_only":     "kept",      // legacy unknown key flows
		"TF_VAR_per_sim_only":    "scenario",  // per-sim only
	}
	for k, want := range cases {
		if got := env[k]; got != want {
			t.Errorf("env[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestTerraformEnvVars_MapValueJSONEncoded(t *testing.T) {
	d := &SimrunDetonator{
		packConfig: config.PackConfig{
			Parameters: map[string]any{
				"default_tags": map[string]any{"team": "secops"},
			},
		},
	}
	env := d.terraformEnvVars("exec-1")
	got := env["TF_VAR_default_tags"]
	// JSON object literal is HCL-compatible as a map<string,string>.
	if got != `{"team":"secops"}` {
		t.Errorf("TF_VAR_default_tags = %q, want JSON-encoded object", got)
	}
}

func TestFormatTFVar(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{"hello", "hello"},
		{true, "true"},
		{false, "false"},
		{42, "42"},
		{map[string]string{"a": "1"}, `{"a":"1"}`},
		{[]string{"a", "b"}, `["a","b"]`},
	}
	for _, tt := range tests {
		if got := formatTFVar(tt.in); got != tt.want {
			t.Errorf("formatTFVar(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
