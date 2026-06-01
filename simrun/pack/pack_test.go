package pack

import (
	"strings"
	"testing"
)

// resetRegistry clears the package-level simulation registry so tests can
// re-register the same scope.id pair without tripping the duplicate guard.
func resetRegistry(t *testing.T) {
	t.Helper()
	for k := range registry {
		delete(registry, k)
	}
}

func TestRegister_RequiredOutputsHappyPath(t *testing.T) {
	resetRegistry(t)
	defer resetRegistry(t)

	Register(Simulation{
		ID:    "happy",
		Scope: "test",
		Terraform: `
output "bucket_arn" { value = "x" }
output "extra"      { value = "y" }
`,
		RequiredOutputs: []string{"bucket_arn"},
	})

	if _, ok := GetSimulation("test.happy"); !ok {
		t.Fatal("expected simulation to be registered")
	}
}

func TestRegister_RequiredOutputsNilSkipsValidation(t *testing.T) {
	resetRegistry(t)
	defer resetRegistry(t)

	// Garbage HCL would fail the parser if validation ran. With nil
	// RequiredOutputs the SDK must skip validation entirely.
	Register(Simulation{
		ID:        "skip",
		Scope:     "test",
		Terraform: `this is not valid hcl {{{`,
	})

	if _, ok := GetSimulation("test.skip"); !ok {
		t.Fatal("expected simulation to be registered")
	}
}

func TestRegister_RequiredOutputsMissingPanics(t *testing.T) {
	resetRegistry(t)
	defer resetRegistry(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is not a string: %T %v", r, r)
		}
		for _, want := range []string{"test.missing", "missing terraform outputs", "bucket_arn", "declared in HCL", "other"} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic message %q does not contain %q", msg, want)
			}
		}
	}()

	Register(Simulation{
		ID:    "missing",
		Scope: "test",
		Terraform: `
output "other" { value = "x" }
`,
		RequiredOutputs: []string{"bucket_arn"},
	})
}

func TestRegister_EmptyTerraformWithRequiredOutputsPanics(t *testing.T) {
	resetRegistry(t)
	defer resetRegistry(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg := r.(string)
		for _, want := range []string{"test.empty", "no Terraform body", "RequiredOutputs"} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic message %q does not contain %q", msg, want)
			}
		}
	}()

	Register(Simulation{
		ID:              "empty",
		Scope:           "test",
		Terraform:       "",
		RequiredOutputs: []string{"bucket_arn"},
	})
}

func TestRegister_EmptyTerraformWithoutRequiredOutputsOK(t *testing.T) {
	resetRegistry(t)
	defer resetRegistry(t)

	Register(Simulation{
		ID:        "ok",
		Scope:     "test",
		Terraform: "",
	})

	if _, ok := GetSimulation("test.ok"); !ok {
		t.Fatal("expected simulation to be registered")
	}
}

func TestRegister_UnparseableTerraformPanics(t *testing.T) {
	resetRegistry(t)
	defer resetRegistry(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg := r.(string)
		for _, want := range []string{"test.broken", "failed to parse embedded Terraform"} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic message %q does not contain %q", msg, want)
			}
		}
	}()

	Register(Simulation{
		ID:              "broken",
		Scope:           "test",
		Terraform:       `output "bad {`,
		RequiredOutputs: []string{"anything"},
	})
}

func TestRegister_DuplicatePanicTakesPrecedenceOverValidation(t *testing.T) {
	resetRegistry(t)
	defer resetRegistry(t)

	// First registration with valid contract.
	Register(Simulation{
		ID:    "dup",
		Scope: "test",
		Terraform: `
output "ok" { value = "x" }
`,
		RequiredOutputs: []string{"ok"},
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from duplicate registration")
		}
		msg := r.(string)
		// Must be the duplicate-registration message, not the new
		// missing-output message — the duplicate check runs first.
		if !strings.Contains(msg, "duplicate registration") {
			t.Errorf("expected duplicate-registration panic, got %q", msg)
		}
		if strings.Contains(msg, "missing terraform outputs") {
			t.Errorf("validation panic fired before duplicate check: %q", msg)
		}
	}()

	// Second registration with a deliberately broken RequiredOutputs.
	// The duplicate-ID check inside registerItem must fire first.
	Register(Simulation{
		ID:    "dup",
		Scope: "test",
		Terraform: `
output "ok" { value = "x" }
`,
		RequiredOutputs: []string{"not_declared"},
	})
}
