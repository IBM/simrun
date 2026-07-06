package detonators

import (
	"testing"

	"github.com/IBM/simrun/internal/config"
	"github.com/IBM/simrun/pack"
)

// TestDetonationEnvVars_AWSRegion verifies the detonation env pins
// AWS_REGION/AWS_DEFAULT_REGION to the same aws_region value terraform's
// provider uses. Without this, terraform creates resources in var.aws_region
// (default us-east-1) while the detonation SDK resolves region from the
// container's ambient AWS_REGION, so deletes/lookups hit the wrong region and
// fail with NotFound. Precedence mirrors terraformEnvVars: per-sim > pack-level
// > built-in default.
func TestDetonationEnvVars_AWSRegion(t *testing.T) {
	tests := []struct {
		name       string
		envVars    map[string]string
		packParams map[string]any
		simParams  map[string]any
		want       string
	}{
		{
			name: "unset falls back to built-in default",
			want: pack.DefaultAWSRegion,
		},
		{
			name:       "pack-level value used",
			packParams: map[string]any{"aws_region": "eu-west-1"},
			want:       "eu-west-1",
		},
		{
			name:       "per-sim overrides pack-level",
			packParams: map[string]any{"aws_region": "eu-west-1"},
			simParams:  map[string]any{"aws_region": "ap-south-1"},
			want:       "ap-south-1",
		},
		{
			name:       "param overrides ambient run-env region",
			envVars:    map[string]string{"AWS_REGION": "us-west-2"},
			packParams: map[string]any{"aws_region": "eu-west-1"},
			want:       "eu-west-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &SimrunDetonator{
				packConfig: config.PackConfig{Parameters: tt.packParams},
				params:     tt.simParams,
				envVars:    tt.envVars,
			}
			env := d.detonationEnvVars()
			if env["AWS_REGION"] != tt.want {
				t.Errorf("AWS_REGION = %q, want %q", env["AWS_REGION"], tt.want)
			}
			if env["AWS_DEFAULT_REGION"] != tt.want {
				t.Errorf("AWS_DEFAULT_REGION = %q, want %q", env["AWS_DEFAULT_REGION"], tt.want)
			}
		})
	}
}

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

// The org-wide default tags merge (web.loadPacksFromDB) stores default_tags
// as map[string]string; it must reach terraform identically to the
// map[string]any a pack stores on its own.
func TestTerraformEnvVars_MergedStringMapJSONEncoded(t *testing.T) {
	d := &SimrunDetonator{
		packConfig: config.PackConfig{
			Parameters: map[string]any{
				"default_tags": map[string]string{"owner": "secops"},
			},
		},
	}
	env := d.terraformEnvVars("exec-1")
	if got := env["TF_VAR_default_tags"]; got != `{"owner":"secops"}` {
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
