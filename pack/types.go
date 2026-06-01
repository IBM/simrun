// Package pack provides the SDK for building simulation packs.
package pack

import "context"

// MITREMapping represents MITRE ATT&CK framework mappings for a simulation.
type MITREMapping struct {
	Tactics    []string `json:"tactics"`    // MITRE tactic IDs (e.g., ["TA0040"])
	Techniques []string `json:"techniques"` // MITRE technique IDs (e.g., ["T1496"])
}

// Simulation represents a fully registered simulation with metadata, Terraform, and handlers.
// Use Register() in your simulation's init() function to register simulations.
type Simulation struct {
	// ID is the simulation slug (e.g., "ec2-bitcoin-mining").
	// The SDK generates the full qualified ID (scope.slug) at manifest time.
	ID                        string       `json:"id"`
	Name                      string       `json:"name"`
	Description               string       `json:"description"`
	MITRE                     MITREMapping `json:"mitre"`
	Scope                     string       `json:"scope"` // Required: aws, gcp, azure, generic
	IsSlow                    bool         `json:"is_slow,omitempty"`
	RequiresExternalResources bool         `json:"requires_external_resources,omitempty"`
	ParamsSchema              any          `json:"params_schema,omitempty"`
	Terraform                 string       `json:"-"` // raw HCL content, typically from //go:embed
	// RequiredOutputs lists the Terraform output names that Detonate reads
	// from DetonateInput.TerraformOutputs. The SDK validates these against
	// the embedded Terraform body at Register time and panics if any are
	// missing, so authors find sim/TF contract drift at boot rather than
	// after a real `terraform apply`. Leave nil to skip validation.
	RequiredOutputs []string     `json:"-"`
	Detonate        DetonateFunc `json:"-"`
	Cleanup         CleanupFunc  `json:"-"`
}

// DetonateInput is the wire format for the detonate command input.
// The Simulation field is used by the SDK for routing and can be ignored by
// simulation handlers.
type DetonateInput struct {
	Simulation       string            `json:"simulation"`
	ExecutionID      string            `json:"execution_id"`
	Params           map[string]any    `json:"params"`
	TerraformOutputs map[string]string `json:"terraform_outputs"`
}

// CleanupInput is the wire format for the cleanup command input.
// The Simulation field is used by the SDK for routing and can be ignored by
// simulation handlers.
type CleanupInput struct {
	Simulation       string         `json:"simulation"`
	ExecutionID      string         `json:"execution_id"`
	Params           map[string]any `json:"params"`
	DetonationResult *Result        `json:"detonation_result,omitempty"`
}

// Result is the output from a Detonate or Cleanup operation.
type Result struct {
	Status     string         `json:"status"`
	Indicators map[string]any `json:"indicators,omitempty"`
	Error      *Error         `json:"error,omitempty"`
}

// Error represents a simulation execution error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DetonateFunc is the signature for a simulation's Detonate function.
type DetonateFunc func(ctx context.Context, input DetonateInput) (*Result, error)

// CleanupFunc is the signature for a simulation's Cleanup function.
type CleanupFunc func(ctx context.Context, input CleanupInput) error

// PackParam declares a pack-level parameter that the pack exposes to operators.
// Authors register custom params via RegisterPackParams; the SDK ships a fixed
// set of built-in params (default_tags, aws_region, etc.) in addition.
type PackParam struct {
	// Name is the parameter key. Must be unique across custom params and must
	// not collide with a reserved built-in name.
	Name string
	// Type is one of "string", "boolean", "object_string_map".
	Type string
	// Description is human-readable help text shown in the UI.
	Description string
	// Default is the default value used when the operator does not set the
	// param. Must match Type: string for "string", bool for "boolean",
	// map[string]string for "object_string_map".
	Default any
	// Required marks the param as mandatory. Backend validation rejects a
	// PUT /api/packs/{name}/parameters that omits a required param.
	Required bool
	// Enum, when non-empty, restricts a "string"-typed param's allowed values.
	// Invalid on any non-string type.
	Enum []string
}

// PackParam Type constants.
const (
	PackParamTypeString          = "string"
	PackParamTypeBoolean         = "boolean"
	PackParamTypeObjectStringMap = "object_string_map"
)

// Template represents an injection template with metadata.
type Template struct {
	// ID is the template slug (e.g., "add-group-member").
	// The SDK generates the full qualified ID (scope.slug) at manifest time.
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"` // Required: okta, aws, gcp, azure, generic
	Content     string `json:"-"`     // Raw template content, typically from //go:embed
}

// Status values for results.
const (
	StatusSuccess = "success"
	StatusError   = "error"
)

// Standard error codes.
const (
	ErrCodePermissionDenied = "PERMISSION_DENIED"
	ErrCodeResourceNotFound = "RESOURCE_NOT_FOUND"
	ErrCodeTimeout          = "TIMEOUT"
	ErrCodeInvalidParams    = "INVALID_PARAMS"
	ErrCodeInternalError    = "INTERNAL_ERROR"
)

// SuccessResult creates a successful result with the given indicators.
func SuccessResult(indicators map[string]any) *Result {
	return &Result{
		Status:     StatusSuccess,
		Indicators: indicators,
	}
}

// ErrorResult creates an error result with the given code and message.
func ErrorResult(code, message string) *Result {
	return &Result{
		Status: StatusError,
		Error:  &Error{Code: code, Message: message},
	}
}
