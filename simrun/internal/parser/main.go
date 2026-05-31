package parser

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/IBM/simrun/simrun/internal/collectors"
	"github.com/IBM/simrun/simrun/internal/config"
	"github.com/IBM/simrun/simrun/internal/detonators"
	"github.com/IBM/simrun/simrun/internal/injectors"
	"github.com/IBM/simrun/simrun/internal/matchers"
	"github.com/IBM/simrun/simrun/internal/matchers/datadog"
	"github.com/IBM/simrun/simrun/internal/matchers/elastic"
	packrunner "github.com/IBM/simrun/simrun/internal/packs/runner"
	"github.com/IBM/simrun/simrun/internal/runner"
	"sigs.k8s.io/yaml" // we use this library as it provides a handy "YAMLToJSON" function
)

// ParseOptions contains additional options for parsing scenarios.
// SSH credentials arrive in EnvVars (SR_SSH_HOST/USERNAME/KEY) from the
// scenario's resolved ssh connector — see web.ScenarioService.Run.
type ParseOptions struct {
	Packs            []config.PackConfig
	EnvVars          map[string]string
	DataDir          string
	TerraformVersion string
	PackLogsEnabled  bool
}

// findPackByName returns the matching PackConfig from the parser's pack
// snapshot, or nil if not found.
func findPackByName(packs []config.PackConfig, name string) *config.PackConfig {
	for i := range packs {
		if packs[i].Name == name {
			return &packs[i]
		}
	}
	return nil
}

// ParseResult contains the parsed scenarios and top-level configuration.
type ParseResult struct {
	Scenarios []*runner.Scenario
	Targets map[string]string // cloud type → connector name (e.g. "aws" → "prod-aws")
}

// Parse turns a YAML input string into a list of Simrun scenarios
func Parse(yamlInput []byte) ([]*runner.Scenario, error) {
	result, err := ParseWithOptions(yamlInput, &ParseOptions{})
	if err != nil {
		return nil, err
	}
	return result.Scenarios, nil
}

// ParseWithOptions turns a YAML input string into a ParseResult with additional options
func ParseWithOptions(yamlInput []byte, opts *ParseOptions) (*ParseResult, error) {
	jsonInput, err := yaml.YAMLToJSON(yamlInput)
	if err != nil {
		return nil, fmt.Errorf("unable to convert input YAML to JSON: %v", err)
	}

	parsed := SimrunSchemaJson{}
	if err := parsed.UnmarshalJSON(jsonInput); err != nil {
		return nil, fmt.Errorf("unable to parse input: %v", err)
	}

	scenarios, err := buildScenarios(&parsed, opts)
	if err != nil {
		return nil, err
	}

	targets := extractTargets(parsed.Targets)

	return &ParseResult{
		Scenarios: scenarios,
		Targets:   targets,
	}, nil
}

func buildScenarios(parsed *SimrunSchemaJson, opts *ParseOptions) ([]*runner.Scenario, error) {
	if len(parsed.Scenarios) == 0 {
		return nil, fmt.Errorf("input file has no scenarios defined")
	}

	scenarios := make([]*runner.Scenario, 0, len(parsed.Scenarios))
	for _, parsedScenario := range parsed.Scenarios {
		if !parsedScenario.Enabled {
			continue
		}
		scenario, err := buildScenario(parsedScenario, parsed.Metadata, opts)
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, scenario)
	}
	if len(scenarios) == 0 {
		return nil, fmt.Errorf("all scenarios are disabled")
	}
	return scenarios, nil
}

func buildScenario(parsedScenario SimrunSchemaJsonScenariosElem, metadata *SimrunSchemaJsonMetadata, opts *ParseOptions) (*runner.Scenario, error) {
	scenario := &runner.Scenario{Name: parsedScenario.Name}

	// Validate scenario has detonation or injection
	if parsedScenario.Detonate == nil && parsedScenario.Inject == nil {
		return nil, fmt.Errorf("scenario '%s' has no detonation or injection defined", parsedScenario.Name)
	}

	// Detonation
	if parsedScenario.Detonate != nil {
		detonator, err := createDetonator(parsedScenario.Detonate, parsedScenario.Name, opts)
		if err != nil {
			return nil, err
		}
		scenario.Detonator = detonator
	}

	// Injection
	if parsedScenario.Inject != nil {
		injector, err := createInjector(parsedScenario.Inject, parsedScenario.Name, opts)
		if err != nil {
			return nil, err
		}
		scenario.Injector = injector
	}

	// Collector
	if parsedScenario.Collect != nil && parsedScenario.Collect.ElasticCollector != nil {
		scenario.Collector = createElasticCollector(parsedScenario.Collect.ElasticCollector, parsedScenario.Name, opts.EnvVars)
	}

	// Indicators
	if parsedScenario.Indicators != nil {
		scenario.Indicators = &runner.Indicators{
			TerraformOutput: parsedScenario.Indicators.TerraformOutput,
			Static:          parsedScenario.Indicators.Static,
		}
	}

	// Assertions and timeout
	if len(parsedScenario.Expectations) == 0 {
		return nil, fmt.Errorf("scenario '%s' has no assertions defined", parsedScenario.Name)
	}

	assertions, err := buildAssertions(parsedScenario.Expectations, opts.EnvVars)
	if err != nil {
		return nil, err
	}
	scenario.Assertions = assertions

	timeout, err := time.ParseDuration(parsedScenario.Expectations[0].Timeout)
	if err != nil {
		return nil, fmt.Errorf("scenario '%s' has an invalid timeout '%s': '%v'", parsedScenario.Name, parsedScenario.Expectations[0].Timeout, err)
	}
	scenario.Timeout = timeout

	// Metadata
	if metadata != nil {
		scenario.Metadata = &runner.Metadata{
			Name:        *metadata.Name,
			Description: *metadata.Description,
		}
	}

	return scenario, nil
}

func createInjector(inject *SimrunSchemaJsonScenariosElemInject, scenarioName string, opts *ParseOptions) (injectors.Injector, error) {
	if inject.ElasticInjector == nil {
		return nil, nil
	}

	elasticInjector := inject.ElasticInjector
	injector := &injectors.ElasticInjector{EnvVars: opts.EnvVars}

	// Collect pack names that have templates
	packNames := map[string]bool{}
	for _, doc := range elasticInjector.Documents {
		if doc.Template != nil {
			if doc.Pack == nil {
				return nil, fmt.Errorf("scenario '%s': document specifies template '%s' without pack", scenarioName, *doc.Template)
			}
			packNames[*doc.Pack] = true
		}
	}

	// Fetch templates if needed
	if len(packNames) > 0 {
		templateCache, err := fetchPackTemplates(packNames, opts)
		if err != nil {
			return nil, fmt.Errorf("scenario '%s': %w", scenarioName, err)
		}
		injector.TemplateCache = templateCache
	}

	// Convert documents
	injector.Documents = make([]injectors.ElasticInjectorDocument, 0, len(elasticInjector.Documents))
	for _, doc := range elasticInjector.Documents {
		injectorDoc := injectors.ElasticInjectorDocument{
			Index: doc.Index,
			Vars:  doc.Vars,
		}
		if doc.File != nil {
			injectorDoc.File = *doc.File
		}
		if doc.Template != nil {
			injectorDoc.Template = *doc.Template
		}
		if doc.Pack != nil {
			injectorDoc.Pack = *doc.Pack
		}
		injector.Documents = append(injector.Documents, injectorDoc)
	}

	return injector, nil
}

func createElasticCollector(elasticCollector *ElasticCollectorSchemaJson, scenarioName string, envVars map[string]string) collectors.Collector {
	collectorConfig := collectors.LoadElasticCollectorConfig(envVars)
	scenarioConfig := &collectors.ElasticCollectorScenarioConfig{
		Index:            elasticCollector.Index,
		AdditionalFields: elasticCollector.AdditionalFields,
	}
	return collectors.NewElasticCollector(collectorConfig, scenarioConfig, scenarioName)
}


// createDetonator creates the appropriate detonator based on the scenario configuration
func createDetonator(detonate *SimrunSchemaJsonScenariosElemDetonate, scenarioName string, opts *ParseOptions) (detonators.Detonator, error) {
	if detonate.AwsCliDetonator != nil {
		return detonators.NewAWSCLIDetonator(*detonate.AwsCliDetonator.Script), nil
	}

	if detonate.SimrunDetonator != nil {
		return createSimrunDetonator(detonate.SimrunDetonator, scenarioName, opts)
	}

	return nil, nil
}

// createSimrunDetonator creates a SimrunDetonator with proper configuration
func createSimrunDetonator(simrunDet *SimrunDetonatorSchemaJson, scenarioName string, opts *ParseOptions) (detonators.Detonator, error) {
	packConfig := findPackByName(opts.Packs, simrunDet.Pack)
	if packConfig == nil {
		return nil, fmt.Errorf("scenario '%s' references unknown pack '%s'", scenarioName, simrunDet.Pack)
	}

	// Convert params to map[string]any
	params := make(map[string]any, len(simrunDet.Params))
	for k, v := range simrunDet.Params {
		params[k] = v
	}

	detonator, err := detonators.NewSimrunDetonator(simrunDet.Simulation, *packConfig, params, detonators.DetonatorOptions{
		DataDir:          opts.DataDir,
		TerraformVersion: opts.TerraformVersion,
		PackLogsEnabled:  opts.PackLogsEnabled,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create pack detonator for scenario '%s': %w", scenarioName, err)
	}

	return detonator, nil
}

// fetchPackTemplates fetches manifests from referenced packs and extracts templates into a cache.
func fetchPackTemplates(packNames map[string]bool, opts *ParseOptions) (map[string]string, error) {
	factory, err := packrunner.NewFactory(opts.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create pack runner factory: %w", err)
	}

	cache := make(map[string]string)
	ctx := context.Background()

	for packName := range packNames {
		packConfig := findPackByName(opts.Packs, packName)
		if packConfig == nil {
			return nil, fmt.Errorf("pack '%s' not found in configuration", packName)
		}

		manifest, err := factory.GetManifest(ctx, *packConfig, packConfig.Parameters, opts.EnvVars)
		if err != nil {
			return nil, fmt.Errorf("failed to get manifest for pack '%s': %w", packName, err)
		}

		for _, tmpl := range manifest.Templates {
			decoded, err := base64.StdEncoding.DecodeString(tmpl.Content)
			if err != nil {
				return nil, fmt.Errorf("failed to decode template '%s' from pack '%s': %w", tmpl.ID, packName, err)
			}
			cache[tmpl.ID] = string(decoded)
		}
	}

	return cache, nil
}

// extractTargets converts the parsed targets struct into a map of cloud type → connector name.
func extractTargets(target *SimrunSchemaJsonTargets) map[string]string {
	if target == nil {
		return nil
	}
	result := make(map[string]string)
	if target.Aws != nil {
		result["aws"] = *target.Aws
	}
	if target.Gcp != nil {
		result["gcp"] = *target.Gcp
	}
	if target.Azure != nil {
		result["azure"] = *target.Azure
	}
	if target.Kubernetes != nil {
		result["kubernetes"] = *target.Kubernetes
	}
	if target.Ssh != nil {
		result["ssh"] = *target.Ssh
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// buildAssertions creates assertion matchers from expectations
func buildAssertions(expectations []SimrunSchemaJsonScenariosElemExpectationsElem, envVars map[string]string) ([]matchers.AlertGeneratedMatcher, error) {
	var assertions []matchers.AlertGeneratedMatcher

	for _, expectation := range expectations {
		if datadogMatcher := expectation.DatadogSecuritySignal; datadogMatcher != nil {
			assertion := datadog.DatadogSecuritySignal(datadogMatcher.Name, envVars)
			if severity := datadogMatcher.Severity; severity != nil {
				assertion.WithSeverity(*severity)
			}
			assertions = append(assertions, assertion)
		}

		if elasticMatcher := expectation.ElasticSecurityAlert; elasticMatcher != nil {
			assertion, err := elastic.ElasticSecurityAlert(elasticMatcher.Name, envVars)
			if err != nil {
				return nil, fmt.Errorf("failed to create Elastic Security alert matcher: %w", err)
			}
			if severity := elasticMatcher.Severity; severity != nil {
				assertion.WithSeverity(string(*severity))
			}
			assertions = append(assertions, assertion)
		}
	}

	return assertions, nil
}
