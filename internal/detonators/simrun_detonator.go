package detonators

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IBM/simrun/internal/config"
	"github.com/IBM/simrun/internal/packs/executor"
	"github.com/IBM/simrun/internal/packs/runner"
	"github.com/IBM/simrun/internal/packs/terraform"
	"github.com/IBM/simrun/pack"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
)

// SimrunDetonator detonates attack simulations using simulation packs.
type SimrunDetonator struct {
	simulationID    string
	packConfig      config.PackConfig
	params          map[string]any
	runnerFactory   *runner.Factory
	tfManager       *terraform.Manager
	packLogsEnabled bool
	runID           string // optional run ID for structured logging
	statusCallback  func(phase string)
	envVars         map[string]string // run-specific env vars for credential isolation

	// Cached values after resolution
	manifest   *pack.ManifestResponse
	simulation *pack.SimulationManifest
}

// DetonatorOptions carries the configuration NewSimrunDetonator needs from
// its caller. Populated by the web layer from Bootstrap+AppConfig.
type DetonatorOptions struct {
	DataDir          string
	TerraformVersion string
	PackLogsEnabled  bool
}

// SetRunID sets the run ID for structured log routing.
func (d *SimrunDetonator) SetRunID(runID string) {
	d.runID = runID
}

// SetEnvVars sets run-specific environment variables for credential isolation.
func (d *SimrunDetonator) SetEnvVars(envVars map[string]string) {
	d.envVars = envVars
}

// NewSimrunDetonator creates a new SimrunDetonator for the given simulation.
func NewSimrunDetonator(simulationID string, packConfig config.PackConfig, params map[string]any, opts DetonatorOptions) (*SimrunDetonator, error) {
	factory, err := runner.NewFactory(opts.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create runner factory: %w", err)
	}

	tfManager, err := terraform.NewManager(opts.DataDir, opts.TerraformVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform manager: %w", err)
	}

	return &SimrunDetonator{
		simulationID:    simulationID,
		packConfig:      packConfig,
		params:          params,
		runnerFactory:   factory,
		tfManager:       tfManager,
		packLogsEnabled: opts.PackLogsEnabled,
	}, nil
}

// SetStatusCallback sets a callback for reporting phase transitions during detonation.
func (d *SimrunDetonator) SetStatusCallback(callback func(phase string)) {
	d.statusCallback = callback
}

func (d *SimrunDetonator) reportPhase(phase string) {
	if d.statusCallback != nil {
		d.statusCallback(phase)
	}
}

// log returns a logrus entry with run_id included (if set).
func (d *SimrunDetonator) log() *logrus.Entry {
	if d.runID != "" {
		return logrus.WithField("run_id", d.runID)
	}
	return logrus.NewEntry(logrus.StandardLogger())
}

// Detonate executes the attack simulation via the pack.
func (d *SimrunDetonator) Detonate() (map[string]string, error) {
	ctx := context.Background()
	executionID, err := gonanoid.New()
	if err != nil {
		return nil, fmt.Errorf("failed to generate execution ID: %w", err)
	}

	d.setupLoggingAndTerraform(executionID)

	// Get manifest and find simulation
	simulation, err := d.resolveSimulation(ctx)
	if err != nil {
		return nil, err
	}

	d.logSimulationInfo(simulation)

	// Handle Terraform if simulation requires it
	if simulation.Terraform != "" {
		d.reportPhase("warmup")
	}
	terraformOutputs, tfWorkDir, err := d.setupTerraform(ctx, executionID, simulation)
	if err != nil {
		return nil, err
	}
	if tfWorkDir != "" {
		defer d.cleanupTerraform(ctx, executionID, tfWorkDir)
	}

	// Execute the simulation
	d.reportPhase("detonating")
	detonateOutput, err := d.executeSimulation(ctx, executionID, terraformOutputs)
	if err != nil {
		return nil, err
	}

	// Run custom cleanup if needed
	if simulation.HasCustomCleanup {
		d.runCustomCleanup(ctx, executionID, detonateOutput)
	}

	// Build and return result
	return d.buildResult(executionID, detonateOutput, terraformOutputs), nil
}

// resolveSimulation gets the manifest and finds the target simulation
func (d *SimrunDetonator) resolveSimulation(ctx context.Context) (*pack.SimulationManifest, error) {
	manifest, err := d.runnerFactory.GetManifest(ctx, d.packConfig, d.packConfig.Parameters, d.envVars)
	if err != nil {
		return nil, fmt.Errorf("failed to get pack manifest: %w", err)
	}
	d.manifest = manifest

	simulation, err := d.findSimulation(manifest)
	if err != nil {
		return nil, err
	}
	d.simulation = simulation

	return simulation, nil
}

// setupTerraform handles Terraform setup and execution if required
func (d *SimrunDetonator) setupTerraform(ctx context.Context, executionID string, simulation *pack.SimulationManifest) (map[string]string, string, error) {
	if simulation.Terraform == "" {
		return nil, "", nil
	}

	tfWorkDir, err := d.tfManager.Setup(ctx, executionID, simulation.Terraform)
	if err != nil {
		return nil, "", fmt.Errorf("failed to setup terraform: %w", err)
	}

	tfEnvVars := d.terraformEnvVars(executionID)
	terraformOutputs, err := d.tfManager.Apply(ctx, tfWorkDir, tfEnvVars)
	if err != nil {
		// Clean up on error
		d.cleanupTerraform(ctx, executionID, tfWorkDir)
		return nil, "", fmt.Errorf("failed to apply terraform: %w", err)
	}

	return terraformOutputs, tfWorkDir, nil
}

// cleanupTerraform destroys resources and cleans up working directory.
// If destroy fails, the working directory (including state) is preserved
// so resources can be manually cleaned up with terraform destroy.
func (d *SimrunDetonator) cleanupTerraform(ctx context.Context, executionID string, tfWorkDir string) {
	tfEnvVars := d.terraformEnvVars(executionID)
	if destroyErr := d.tfManager.Destroy(ctx, tfWorkDir, tfEnvVars); destroyErr != nil {
		d.log().WithField("error", destroyErr.Error()).WithField("work_dir", tfWorkDir).
			Error("Failed to destroy terraform resources, preserving state for manual cleanup")
		return
	}
	if cleanupErr := d.tfManager.Cleanup(tfWorkDir); cleanupErr != nil {
		d.log().WithError(cleanupErr).Warn("Failed to cleanup terraform working directory")
	}
}

// terraformEnvVars returns environment variables for Terraform execution,
// including TF_APPEND_USER_AGENT to identify simrun requests in cloud provider logs.
// Run-specific env vars (cloud credentials) are included so they override the
// process environment in terraform's buildEnvironment merge.
//
// Pack-level parameters (from packs.parameters in the DB) and per-sim
// scenario parameters are both promoted to TF_VAR_<key> env vars. When
// both scopes set the same key, the per-sim value wins. All keys from
// the pack-level map flow through, including those not declared in
// params_schema, so legacy values continue to reach terraform.
func (d *SimrunDetonator) terraformEnvVars(executionID string) map[string]string {
	vars := make(map[string]string, len(d.envVars)+len(d.packConfig.Parameters)+len(d.params)+1)
	for k, v := range d.envVars {
		vars[k] = v
	}
	vars["TF_APPEND_USER_AGENT"] = pack.UserAgent(executionID)
	// Pack-level first, then per-sim so per-sim overrides on key collision.
	for k, v := range d.packConfig.Parameters {
		vars[fmt.Sprintf("TF_VAR_%s", k)] = formatTFVar(v)
	}
	for k, v := range d.params {
		vars[fmt.Sprintf("TF_VAR_%s", k)] = formatTFVar(v)
	}
	return vars
}

// formatTFVar renders a parameter value into the string form Terraform
// accepts for TF_VAR_*. Map and slice values are JSON-encoded so map/list
// typed Terraform variables receive a parseable HCL/JSON literal.
func formatTFVar(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	case map[string]any, map[string]string, []any, []string:
		raw, err := json.Marshal(typed)
		if err == nil {
			return string(raw)
		}
		return fmt.Sprintf("%v", typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// executeSimulation runs the pack's detonate command
func (d *SimrunDetonator) executeSimulation(ctx context.Context, executionID string, terraformOutputs map[string]string) (*pack.Result, error) {
	exec, packRunner, err := d.createExecutor(ctx, executionID)
	if err != nil {
		return nil, err
	}
	defer packRunner.Close()

	detonateInput := &pack.DetonateInput{
		Simulation:       d.simulationID,
		ExecutionID:      executionID,
		Params:           d.params,
		TerraformOutputs: terraformOutputs,
	}

	detonateOutput, err := exec.Detonate(ctx, detonateInput)
	if err != nil {
		return nil, fmt.Errorf("detonation failed: %w", err)
	}

	if detonateOutput.Status == pack.StatusError {
		errMsg := d.formatDetonationError(detonateOutput.Error)
		return nil, fmt.Errorf("detonation failed: %s", errMsg)
	}

	return detonateOutput, nil
}

// runCustomCleanup executes the pack's cleanup command
func (d *SimrunDetonator) runCustomCleanup(ctx context.Context, executionID string, detonateOutput *pack.Result) {
	exec, packRunner, err := d.createExecutor(ctx, executionID)
	if err != nil {
		d.log().WithError(err).Warn("Failed to create runner for cleanup")
		return
	}
	defer packRunner.Close()

	cleanupInput := &pack.CleanupInput{
		Simulation:       d.simulationID,
		ExecutionID:      executionID,
		Params:           d.params,
		DetonationResult: detonateOutput,
	}

	cleanupOutput, err := exec.Cleanup(ctx, cleanupInput)
	if err != nil {
		d.log().WithError(err).Warn("Custom cleanup failed")
		return
	}

	if cleanupOutput.Status == pack.StatusError {
		errMsg := d.formatDetonationError(cleanupOutput.Error)
		d.log().WithField("error", errMsg).Warn("Custom cleanup failed")
	}
}

// formatDetonationError formats an error from pack protocol
func (d *SimrunDetonator) formatDetonationError(err *pack.Error) string {
	if err == nil {
		return "unknown error"
	}
	return fmt.Sprintf("%s: %s", err.Code, err.Message)
}

// buildResult constructs the result map with all indicators
func (d *SimrunDetonator) buildResult(executionID string, detonateOutput *pack.Result, terraformOutputs map[string]string) map[string]string {
	result := map[string]string{"execution_id": executionID}

	// Add indicators from detonation output
	for k, v := range detonateOutput.Indicators {
		result[k] = fmt.Sprintf("%v", v)
	}

	// Add terraform outputs directly
	for k, v := range terraformOutputs {
		result[k] = v
	}

	d.log().WithFields(logrus.Fields{
		"execution_id":    executionID,
		"indicator_count": len(result),
	}).Info("Detonation complete")

	return result
}

// String returns a string representation of the detonator.
func (d *SimrunDetonator) String() string {
	return "SimrunDetonator"
}

// SimulationId returns the simulation ID being detonated.
func (d *SimrunDetonator) SimulationId() string {
	return d.simulationID
}

// CloudProvider returns the cloud provider inferred from the simulation ID.
func (d *SimrunDetonator) CloudProvider() string {
	id := strings.ToLower(d.simulationID)
	if strings.Contains(id, "aws") {
		return "aws"
	}
	if strings.Contains(id, "gcp") {
		return "gcp"
	}
	if strings.Contains(id, "azure") {
		return "azure"
	}
	return ""
}

// PackName returns the name of the pack being used.
func (d *SimrunDetonator) PackName() string {
	return d.packConfig.Name
}

// findSimulation finds the simulation in the manifest by ID.
func (d *SimrunDetonator) findSimulation(manifest *pack.ManifestResponse) (*pack.SimulationManifest, error) {
	for i := range manifest.Simulations {
		if manifest.Simulations[i].ID == d.simulationID {
			return &manifest.Simulations[i], nil
		}
	}

	// List available simulations for error message
	available := make([]string, len(manifest.Simulations))
	for i, s := range manifest.Simulations {
		available[i] = s.ID
	}

	return nil, fmt.Errorf("simulation %s not found in pack %s (available: %v)", d.simulationID, d.packConfig.Name, available)
}

// setupLoggingAndTerraform sets up logging and enriches terraform manager
func (d *SimrunDetonator) setupLoggingAndTerraform(executionID string) {
	SetupLogging(logrus.Fields{
		"detonator":    "simrunDetonator",
		"pack":         d.packConfig.Name,
		"simulation":   d.simulationID,
		"execution_id": executionID,
	})

	if d.runID != "" {
		d.tfManager = d.tfManager.WithLogFields(logrus.Fields{
			"run_id":       d.runID,
			"execution_id": executionID,
		})
	}
}

// logSimulationInfo logs information about the found simulation
func (d *SimrunDetonator) logSimulationInfo(simulation *pack.SimulationManifest) {
	d.log().WithFields(logrus.Fields{
		"pack":            d.packConfig.Name,
		"simulation":      simulation.ID,
		"simulation_name": simulation.Name,
		"has_terraform":   simulation.Terraform != "",
		"has_cleanup":     simulation.HasCustomCleanup,
	}).Info("Simulation found in pack manifest")
}

// createExecutor creates a pack runner and executor for simulation execution
func (d *SimrunDetonator) createExecutor(ctx context.Context, executionID string) (*executor.Executor, runner.PackRunner, error) {
	packRunner, err := d.runnerFactory.CreateRunner(ctx, d.packConfig, d.envVars)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pack runner: %w", err)
	}

	exec := executor.NewExecutor(packRunner, d.packLogsEnabled)
	if d.runID != "" {
		exec = exec.WithLogFields(logrus.Fields{
			"run_id":       d.runID,
			"execution_id": executionID,
		})
	}

	return exec, packRunner, nil
}
