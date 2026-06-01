// Package terraform provides programmatic Terraform execution using terraform-exec.
package terraform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/sirupsen/logrus"
)

// Global state for terraform binary (protected by sync.Once)
var (
	terraformOnce    sync.Once
	terraformPath    string
	terraformVersion string
	terraformErr     error
)

// Manager handles Terraform operations for pack simulations.
type Manager struct {
	baseDir          string
	terraformPath    string
	terraformVersion string
	logFields        logrus.Fields
}

// WithLogFields returns a copy of the Manager that includes extra fields in all log entries.
func (m *Manager) WithLogFields(fields logrus.Fields) *Manager {
	return &Manager{
		baseDir:          m.baseDir,
		terraformPath:    m.terraformPath,
		terraformVersion: m.terraformVersion,
		logFields:        fields,
	}
}

// log returns a logrus entry with the manager's extra fields included.
func (m *Manager) log() *logrus.Entry {
	if len(m.logFields) == 0 {
		return logrus.NewEntry(logrus.StandardLogger())
	}
	return logrus.WithFields(m.logFields)
}

// NewManager creates a new Terraform Manager rooted at dataDir.
// It downloads terraform to <dataDir>/bin/ if not already present, or falls back to local terraform.
// The terraform binary is downloaded once and cached for all subsequent Manager instances.
// requestedVersion may be empty to install the latest version.
func NewManager(dataDir, requestedVersion string) (*Manager, error) {
	baseDir := filepath.Join(dataDir, "terraform")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create terraform directory: %w", err)
	}

	// Ensure terraform binary is available (download if needed)
	// Uses sync.Once to prevent race conditions during parallel scenario execution
	terraformOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		terraformPath, terraformVersion, terraformErr = doEnsureTerraform(ctx, dataDir, requestedVersion)
	})

	if terraformErr != nil {
		return nil, terraformErr
	}

	return &Manager{
		baseDir:          baseDir,
		terraformPath:    terraformPath,
		terraformVersion: terraformVersion,
	}, nil
}

// NewManagerWithBaseDir creates a new Manager with a custom base directory.
func NewManagerWithBaseDir(baseDir string, terraformPath string) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create terraform directory: %w", err)
	}

	version := getTerraformVersion(terraformPath)

	return &Manager{
		baseDir:          baseDir,
		terraformPath:    terraformPath,
		terraformVersion: version,
	}, nil
}

// Setup creates a working directory and writes the Terraform files.
// tfContentBase64 is the base64-encoded Terraform file content from the pack manifest.
func (m *Manager) Setup(ctx context.Context, executionID string, tfContentBase64 string) (string, error) {
	workDir := filepath.Join(m.baseDir, executionID)

	// Create working directory
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create working directory: %w", err)
	}

	// Decode base64 content
	tfContent, err := base64.StdEncoding.DecodeString(tfContentBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode terraform content: %w", err)
	}

	// Write main.tf
	mainTFPath := filepath.Join(workDir, "main.tf")
	if err := os.WriteFile(mainTFPath, tfContent, 0644); err != nil {
		return "", fmt.Errorf("failed to write main.tf: %w", err)
	}

	m.log().WithField("execution_id", executionID).WithField("work_dir", workDir).Debug("Terraform working directory setup complete")

	return workDir, nil
}

// Apply runs terraform init + apply and returns the outputs as strings.
func (m *Manager) Apply(ctx context.Context, workDir string, envVars map[string]string) (map[string]string, error) {
	tf, err := m.newTerraform(workDir, envVars)
	if err != nil {
		return nil, err
	}

	m.log().WithField("work_dir", workDir).Info("Running terraform init")

	// Run init
	if err := tf.Init(ctx, tfexec.Upgrade(true)); err != nil {
		return nil, fmt.Errorf("terraform init failed: %w", err)
	}

	m.log().WithField("work_dir", workDir).Info("Running terraform apply")

	// Run apply. Variables are passed as -var options rather than TF_VAR_*
	// env vars because terraform-exec strips the TF_VAR_ prefix in CleanEnv.
	varOpts := terraformVarOptions(envVars, declaredVariables(workDir))
	applyOpts := make([]tfexec.ApplyOption, 0, len(varOpts))
	for _, o := range varOpts {
		applyOpts = append(applyOpts, o)
	}
	if err := tf.Apply(ctx, applyOpts...); err != nil {
		return nil, fmt.Errorf("terraform apply failed: %w", err)
	}

	// Read outputs from state file
	outputs, err := readOutputsFromState(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read terraform outputs: %w", err)
	}

	m.log().WithField("work_dir", workDir).WithField("output_count", len(outputs)).Info("Terraform apply complete")

	return outputs, nil
}

// terraformState represents the relevant parts of a terraform.tfstate file.
type terraformState struct {
	Outputs map[string]terraformOutput `json:"outputs"`
}

type terraformOutput struct {
	Value json.RawMessage `json:"value"`
}

// readOutputsFromState reads terraform.tfstate and extracts outputs as strings.
func readOutputsFromState(workDir string) (map[string]string, error) {
	stateFile := filepath.Join(workDir, "terraform.tfstate")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state terraformState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	result := make(map[string]string, len(state.Outputs))
	for name, output := range state.Outputs {
		var s string
		if err := json.Unmarshal(output.Value, &s); err == nil {
			result[name] = s
		} else {
			result[name] = string(output.Value)
		}
	}

	return result, nil
}

// Destroy runs terraform destroy.
func (m *Manager) Destroy(ctx context.Context, workDir string, envVars map[string]string) error {
	tf, err := m.newTerraform(workDir, envVars)
	if err != nil {
		return err
	}

	m.log().WithField("work_dir", workDir).Info("Running terraform destroy")

	// Destroy must receive the same variables as apply; an unset no-default
	// variable fails destroy too. Pass them as -var options (see Apply).
	varOpts := terraformVarOptions(envVars, declaredVariables(workDir))
	destroyOpts := make([]tfexec.DestroyOption, 0, len(varOpts))
	for _, o := range varOpts {
		destroyOpts = append(destroyOpts, o)
	}
	if err := tf.Destroy(ctx, destroyOpts...); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	m.log().WithField("work_dir", workDir).Info("Terraform destroy complete")

	return nil
}

// Cleanup removes the working directory.
func (m *Manager) Cleanup(workDir string) error {
	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("failed to remove working directory: %w", err)
	}

	m.log().WithField("work_dir", workDir).Debug("Terraform working directory cleaned up")

	return nil
}

// newTerraform creates a configured tfexec.Terraform instance for the given
// working directory. It inherits the current process environment, merges any
// custom env vars, and handles tfexec-managed variables (like TF_APPEND_USER_AGENT)
// through their proper APIs instead of SetEnv.
func (m *Manager) newTerraform(workDir string, customVars map[string]string) (*tfexec.Terraform, error) {
	tf, err := tfexec.NewTerraform(workDir, m.terraformPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform instance: %w", err)
	}

	env := m.buildEnvironment(customVars)

	// Extract TF_APPEND_USER_AGENT before cleaning — it must be set via
	// tf.SetAppendUserAgent() rather than through SetEnv.
	if ua, ok := env["TF_APPEND_USER_AGENT"]; ok {
		if err := tf.SetAppendUserAgent(ua); err != nil {
			return nil, fmt.Errorf("failed to set user agent: %w", err)
		}
	}

	// Remove all tfexec-managed vars from the env map. This also strips every
	// TF_VAR_* key: terraform-exec refuses to set those via the environment
	// (SetEnv would return ErrManualEnvVar). Terraform variables are therefore
	// passed as -var options on apply/destroy instead — see terraformVarOptions.
	env = tfexec.CleanEnv(env)

	if err := tf.SetEnv(env); err != nil {
		return nil, fmt.Errorf("failed to set environment variables: %w", err)
	}

	return tf, nil
}

// terraformVarOptions extracts TF_VAR_<name> entries from env and returns them
// as tfexec.Var options (name=value). terraform-exec prohibits passing
// variables through the process environment — CleanEnv deletes any TF_VAR_*
// key and SetEnv rejects them — so callers that want to set variables must
// supply them as -var options on apply/destroy. Non-TF_VAR_ keys are ignored
// here; they flow to terraform through the process environment.
//
// Only variables present in declared are emitted. Unlike TF_VAR_ env vars
// (which terraform silently ignores when undeclared), a -var for an undeclared
// variable is a hard error, so pack-level params a given sim doesn't declare
// must be dropped. A nil declared map disables filtering (pass everything).
func terraformVarOptions(env map[string]string, declared map[string]bool) []*tfexec.VarOption {
	opts := make([]*tfexec.VarOption, 0, len(env))
	for k, v := range env {
		name, ok := strings.CutPrefix(k, "TF_VAR_")
		if !ok {
			continue
		}
		if declared != nil && !declared[name] {
			continue
		}
		opts = append(opts, tfexec.Var(name+"="+v))
	}
	return opts
}

// declaredVariables parses the Terraform files in workDir and returns the set
// of variable names the configuration declares. Returns nil if no file could
// be parsed, which disables -var filtering so a parse failure can't strip
// legitimately-needed variables. terraform init has already validated these
// files by the time apply/destroy run, so parsing here should not fail.
func declaredVariables(workDir string) map[string]bool {
	matches, _ := filepath.Glob(filepath.Join(workDir, "*.tf"))
	parser := hclparse.NewParser()
	declared := map[string]bool{}
	parsedAny := false
	for _, path := range matches {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		f, diags := parser.ParseHCL(src, path)
		if diags.HasErrors() || f == nil {
			continue
		}
		body, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		parsedAny = true
		for _, b := range body.Blocks {
			if b.Type == "variable" && len(b.Labels) == 1 {
				declared[b.Labels[0]] = true
			}
		}
	}
	if !parsedAny {
		return nil
	}
	return declared
}

// buildEnvironment merges the current process environment with custom variables.
func (m *Manager) buildEnvironment(customVars map[string]string) map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if i := strings.IndexByte(e, '='); i >= 0 {
			env[e[:i]] = e[i+1:]
		}
	}
	for k, v := range customVars {
		env[k] = v
	}
	return env
}

// ensureTerraform ensures a terraform binary is available, downloading if needed.
// Returns the path to the binary and its version.
// doEnsureTerraform is the internal implementation that downloads terraform.
// It should only be called via sync.Once to prevent concurrent downloads.
func doEnsureTerraform(ctx context.Context, dataDir string, requestedVersion string) (string, string, error) {
	installDir := filepath.Join(dataDir, "bin")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create install directory: %w", err)
	}

	managedPath := filepath.Join(installDir, "terraform")

	// Check if we already have the right version installed
	existingVersion := getTerraformVersion(managedPath)
	if existingVersion != "" && existingVersion != "unknown" {
		if requestedVersion == "" {
			// No specific version requested, use existing
			logrus.WithField("path", managedPath).WithField("version", existingVersion).Debug("Using existing managed terraform binary")
			return managedPath, existingVersion, nil
		}
		// Check if existing version matches requested
		if strings.TrimPrefix(existingVersion, "v") == strings.TrimPrefix(requestedVersion, "v") {
			logrus.WithField("path", managedPath).WithField("version", existingVersion).Debug("Using existing managed terraform binary (version matches)")
			return managedPath, existingVersion, nil
		}
		logrus.WithField("existing", existingVersion).WithField("requested", requestedVersion).Info("Terraform version mismatch, downloading requested version")
	}

	// Download terraform using hc-install
	logrus.WithField("requested_version", requestedVersion).Info("Downloading terraform binary")

	var execPath string
	var err error

	if requestedVersion != "" {
		// Download specific version
		v, parseErr := version.NewVersion(strings.TrimPrefix(requestedVersion, "v"))
		if parseErr != nil {
			return "", "", fmt.Errorf("invalid terraform version %q: %w", requestedVersion, parseErr)
		}
		installer := &releases.ExactVersion{
			Product:    product.Terraform,
			Version:    v,
			InstallDir: installDir,
		}
		execPath, err = installer.Install(ctx)
	} else {
		// Download latest version
		installer := &releases.LatestVersion{
			Product:    product.Terraform,
			InstallDir: installDir,
		}
		execPath, err = installer.Install(ctx)
	}

	if err != nil {
		logrus.WithError(err).Warn("Failed to download terraform, falling back to local installation")
		return findLocalTerraform()
	}

	installedVersion := getTerraformVersion(execPath)
	logrus.WithField("path", execPath).WithField("version", installedVersion).Info("Terraform binary downloaded successfully")

	return execPath, installedVersion, nil
}

// findLocalTerraform locates a terraform binary in PATH or common locations.
func findLocalTerraform() (string, string, error) {
	// First check if terraform is in PATH
	path, err := LookPath("terraform")
	if err == nil {
		ver := getTerraformVersion(path)
		return path, ver, nil
	}

	// Check common installation locations
	commonPaths := []string{
		"/usr/local/bin/terraform",
		"/usr/bin/terraform",
		"/opt/homebrew/bin/terraform",
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			ver := getTerraformVersion(p)
			return p, ver, nil
		}
	}

	return "", "", fmt.Errorf("terraform binary not found: download failed and no local installation found")
}

// LookPath is a variable to allow mocking in tests.
var LookPath = func(file string) (string, error) {
	return execLookPath(file)
}

func execLookPath(file string) (string, error) {
	// Check PATH environment variable
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("PATH environment variable is empty")
	}

	for _, dir := range filepath.SplitList(pathEnv) {
		path := filepath.Join(dir, file)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	return "", fmt.Errorf("%s not found in PATH", file)
}

// getTerraformVersion runs terraform version and returns the version string.
func getTerraformVersion(terraformPath string) string {
	cmd := exec.Command(terraformPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	// Parse first line which contains version like "Terraform v1.5.0"
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		version := strings.TrimSpace(lines[0])
		// Remove "Terraform " prefix if present
		version = strings.TrimPrefix(version, "Terraform ")
		return version
	}

	return "unknown"
}
