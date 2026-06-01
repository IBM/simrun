package terraform

import (
	"context"
	"encoding/base64"
	"os/exec"
	"testing"
)

// encodeTF base64-encodes terraform source the way pack manifests deliver it,
// which is what Manager.Setup expects.
func encodeTF(src string) string {
	return base64.StdEncoding.EncodeToString([]byte(src))
}

// mainTFWithRequiredVar declares a variable with no default and echoes it back
// as an output. terraform apply fails with "No value for required variable"
// unless the variable is actually supplied.
const mainTFWithRequiredVar = `
variable "resource_prefix" {
  type = string
}

output "prefix" {
  value = var.resource_prefix
}
`

func TestTerraformVarOptions_ExtractsOnlyDeclaredTFVarKeys(t *testing.T) {
	env := map[string]string{
		"TF_VAR_resource_prefix": "simrun",
		"TF_VAR_aws_vpc_id":      "vpc-123",   // a TF var, but not declared by this sim
		"AWS_REGION":             "us-west-2", // not a TF var: must be ignored
		"TF_APPEND_USER_AGENT":   "simrun",    // not a TF var: must be ignored
	}
	declared := map[string]bool{"resource_prefix": true}

	opts := terraformVarOptions(env, declared)
	// Only declared TF_VAR_ entries become -var options. aws_vpc_id is a pack
	// param this sim doesn't declare; passing it as -var would be a hard error.
	if len(opts) != 1 {
		t.Fatalf("expected exactly 1 var option (declared resource_prefix), got %d", len(opts))
	}

	// nil declared disables filtering (both TF_VAR_ entries pass through).
	if got := len(terraformVarOptions(env, nil)); got != 2 {
		t.Fatalf("expected 2 var options when filtering disabled, got %d", got)
	}
}

// TestApply_PassesTFVarToTerraform is the regression test for the bug where
// pack/scenario parameters never reached terraform: they were built as TF_VAR_*
// env vars, but tfexec.CleanEnv strips the TF_VAR_ prefix, so a no-default
// variable failed apply with "No value for required variable". The variable
// must be supplied so that apply succeeds and the value round-trips to outputs.
func TestApply_PassesTFVarToTerraform(t *testing.T) {
	tfPath, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("terraform binary not on PATH; skipping integration test")
	}

	workDir := t.TempDir()
	m, err := NewManagerWithBaseDir(workDir, tfPath)
	if err != nil {
		t.Fatalf("NewManagerWithBaseDir: %v", err)
	}

	ctx := context.Background()
	tfWorkDir, err := m.Setup(ctx, "exec-test", encodeTF(mainTFWithRequiredVar))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	envVars := map[string]string{
		"TF_VAR_resource_prefix": "simrun",
		// A pack-level param this sim does not declare. terraform errors on a
		// -var for an undeclared variable, so it must be filtered out rather
		// than passed through (regression: "Value for undeclared variable").
		"TF_VAR_aws_vpc_id": "vpc-0123456789",
	}

	outputs, err := m.Apply(ctx, tfWorkDir, envVars)
	if err != nil {
		t.Fatalf("Apply should succeed and ignore the undeclared pack param, got: %v", err)
	}
	if got := outputs["prefix"]; got != "simrun" {
		t.Fatalf("expected output prefix=simrun (the supplied variable), got %q", got)
	}

	// Destroy must also receive the variable, or it fails the same way.
	if err := m.Destroy(ctx, tfWorkDir, envVars); err != nil {
		t.Fatalf("Destroy should succeed with the variable supplied, got: %v", err)
	}
}
