package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/simrun/internal/detonators"
	"github.com/IBM/simrun/internal/injectors"
	"github.com/IBM/simrun/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "scenarios", name))
	require.NoError(t, err)
	return data
}

func TestParse_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		file      string
		opts      *ParseOptions
		wantErr   string
		assertScn func(t *testing.T, sc *runner.Scenario)
	}{
		{
			name: "aws cli detonator",
			file: "aws-cli-detonator.yaml",
			assertScn: func(t *testing.T, sc *runner.Scenario) {
				_, ok := sc.Detonator.(*detonators.AWSCLIDetonator)
				assert.True(t, ok, "expected AWSCLIDetonator, got %T", sc.Detonator)
			},
		},
		{
			name: "elastic injector",
			file: "elastic-injector.yaml",
			assertScn: func(t *testing.T, sc *runner.Scenario) {
				inj, ok := sc.Injector.(*injectors.ElasticInjector)
				require.True(t, ok, "expected ElasticInjector, got %T", sc.Injector)
				require.Len(t, inj.Documents, 1)
				assert.Equal(t, "logs-test", inj.Documents[0].Index)
				assert.Equal(t, "doc.json", inj.Documents[0].File)
			},
		},
		{
			name: "elastic collector",
			file: "elastic-collector.yaml",
			assertScn: func(t *testing.T, sc *runner.Scenario) {
				assert.NotNil(t, sc.Collector)
			},
		},
		{
			name:    "empty scenarios fails",
			file:    "empty-scenarios.yaml",
			wantErr: "no scenarios defined",
		},
		{
			name:    "all disabled fails",
			file:    "all-disabled.yaml",
			wantErr: "all scenarios are disabled",
		},
		{
			name:    "missing detonate and inject fails",
			file:    "missing-detonate-and-inject.yaml",
			wantErr: "no detonation or injection",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			yaml := loadFixture(t, tc.file)
			opts := tc.opts
			if opts == nil {
				opts = &ParseOptions{}
			}

			result, err := ParseWithOptions(yaml, opts)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotEmpty(t, result.Scenarios)
			if tc.assertScn != nil {
				tc.assertScn(t, result.Scenarios[0])
			}
		})
	}
}

func TestParse_Targets(t *testing.T) {
	cases := []struct {
		name string
		file string
		want map[string]string
	}{
		{
			name: "no targets block",
			file: "aws-cli-detonator.yaml",
			want: nil,
		},
		{
			name: "aws only",
			file: "targets-aws-only.yaml",
			want: map[string]string{"aws": "prod-aws"},
		},
		{
			name: "all clouds",
			file: "targets-all.yaml",
			want: map[string]string{
				"aws":        "prod-aws",
				"gcp":        "prod-gcp",
				"azure":      "prod-azure",
				"kubernetes": "prod-k8s",
				"ssh":        "prod-ssh",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseWithOptions(loadFixture(t, tc.file), &ParseOptions{})
			require.NoError(t, err)
			assert.Equal(t, tc.want, result.Targets)
		})
	}
}
