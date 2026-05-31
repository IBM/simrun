package pack

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// containsAssignment matches `<attr><whitespace>=<whitespace><value>` in
// hclwrite output, which formats attributes with aligned `=` signs.
func containsAssignment(s, attr, value string) bool {
	re := regexp.MustCompile(regexp.QuoteMeta(attr) + `\s*=\s*` + regexp.QuoteMeta(value))
	return re.MatchString(s)
}

func TestApplyBuiltinPackParams_AWSRegionAndTags(t *testing.T) {
	tf := `
provider "aws" {}

resource "aws_s3_bucket" "b" {
  bucket = "foo"
}
`
	out := string(applyBuiltinPackParams([]byte(tf), "test.sim", nil))

	// aws_region injected into provider
	if !containsAssignment(out, "region", "var.aws_region") {
		t.Errorf("expected region = var.aws_region, got:\n%s", out)
	}
	// default_tags block injected referencing var.default_tags
	if !strings.Contains(out, "default_tags") {
		t.Errorf("expected default_tags block, got:\n%s", out)
	}
	if !containsAssignment(out, "tags", "var.default_tags") {
		t.Errorf("expected tags = var.default_tags, got:\n%s", out)
	}
	// variable blocks inserted
	if !strings.Contains(out, `variable "aws_region"`) {
		t.Errorf("expected variable \"aws_region\" block, got:\n%s", out)
	}
	if !strings.Contains(out, `variable "default_tags"`) {
		t.Errorf("expected variable \"default_tags\" block, got:\n%s", out)
	}
	// simulation_id in default_tags variable default
	if !strings.Contains(out, "simrun_simulation_id") {
		t.Errorf("expected simrun_simulation_id in default_tags default, got:\n%s", out)
	}
	// The simulation id is dot-sanitized so the same default_tags map is a
	// valid GCP label value (GCP forbids dots in labels). The dotted form
	// must not survive.
	if !strings.Contains(out, `"test_sim"`) {
		t.Errorf("expected sanitized simulation id \"test_sim\" in default, got:\n%s", out)
	}
	if strings.Contains(out, `"test.sim"`) {
		t.Errorf("dotted simulation id must be sanitized, got:\n%s", out)
	}
}

func TestApplyBuiltinPackParams_AWSExistingRegionUntouched(t *testing.T) {
	tf := `
provider "aws" {
  region = "ap-southeast-2"
}
`
	out := string(applyBuiltinPackParams([]byte(tf), "test.sim", nil))
	if !containsAssignment(out, "region", `"ap-southeast-2"`) {
		t.Errorf("existing region literal should be preserved, got:\n%s", out)
	}
	if containsAssignment(out, "region", "var.aws_region") {
		t.Errorf("existing region should not be rewritten to var, got:\n%s", out)
	}
}

func TestApplyBuiltinPackParams_DefaultTagsMergeWithExisting(t *testing.T) {
	tf := `
provider "aws" {
  default_tags {
    tags = {
      existing = "value"
    }
  }
}
`
	out := string(applyBuiltinPackParams([]byte(tf), "test.sim", nil))
	if !strings.Contains(out, "merge(var.default_tags") {
		t.Errorf("expected merge(var.default_tags, ...), got:\n%s", out)
	}
	if !strings.Contains(out, "existing") || !strings.Contains(out, `"value"`) {
		t.Errorf("expected existing tags preserved in merge, got:\n%s", out)
	}
}

func TestApplyBuiltinPackParams_MultipleAWSProviders(t *testing.T) {
	tf := `
provider "aws" {
  alias = "primary"
}

provider "aws" {
  alias  = "secondary"
  region = "us-west-2"
}
`
	out := string(applyBuiltinPackParams([]byte(tf), "test.sim", nil))
	// Primary has no region: should get var.aws_region.
	if !containsAssignment(out, "region", "var.aws_region") {
		t.Errorf("primary should get var.aws_region, got:\n%s", out)
	}
	// Secondary keeps its literal.
	if !containsAssignment(out, "region", `"us-west-2"`) {
		t.Errorf("secondary should keep literal region, got:\n%s", out)
	}
}

func TestApplyBuiltinPackParams_GCP(t *testing.T) {
	tf := `
provider "google" {}

resource "google_storage_bucket" "b" {
  name = "x"
}
`
	out := string(applyBuiltinPackParams([]byte(tf), "test.sim", nil))
	if !containsAssignment(out, "region", "var.gcp_region") {
		t.Errorf("expected region = var.gcp_region, got:\n%s", out)
	}
	if !containsAssignment(out, "default_labels", "var.default_tags") {
		t.Errorf("expected default_labels = var.default_tags, got:\n%s", out)
	}
	if !strings.Contains(out, `variable "gcp_region"`) {
		t.Errorf("expected variable \"gcp_region\" block, got:\n%s", out)
	}
	if strings.Contains(out, `variable "gcp_project"`) {
		t.Errorf("gcp_project should not be a built-in, got:\n%s", out)
	}
}

func TestApplyBuiltinPackParams_AzureResourceGroup(t *testing.T) {
	tf := `
resource "azurerm_resource_group" "rg1" {
  name = "rg1"
}

resource "azurerm_resource_group" "rg2" {
  name     = "rg2"
  location = "westus2"
}
`
	out := string(applyBuiltinPackParams([]byte(tf), "test.sim", nil))
	if !containsAssignment(out, "location", "var.azure_location") {
		t.Errorf("expected location = var.azure_location for rg1, got:\n%s", out)
	}
	if !containsAssignment(out, "location", `"westus2"`) {
		t.Errorf("rg2's literal location should be preserved, got:\n%s", out)
	}
	if !strings.Contains(out, `variable "azure_location"`) {
		t.Errorf("expected variable \"azure_location\" block, got:\n%s", out)
	}
}

func TestApplyBuiltinPackParams_NoOpWhenBlockAbsent(t *testing.T) {
	tf := `
resource "aws_s3_bucket" "b" {
  bucket = "foo"
}
`
	out := string(applyBuiltinPackParams([]byte(tf), "test.sim", nil))
	// No provider blocks → no aws_region/gcp_*/azure_location variable inserted.
	for _, want := range []string{
		`variable "aws_region"`,
		`variable "gcp_region"`,
		`variable "azure_location"`,
	} {
		if strings.Contains(out, want) {
			t.Errorf("expected no %s when no matching block, got:\n%s", want, out)
		}
	}
	// No provider block means no default_tags variable either (rewriteDefaultTags
	// won't touch aws_s3_bucket directly).
	if strings.Contains(out, `variable "default_tags"`) {
		t.Errorf("expected no default_tags variable when no provider/azure block, got:\n%s", out)
	}
}

func TestApplyBuiltinPackParams_DefaultTagsVariableCarriesOperatorTagsPlusSimID(t *testing.T) {
	tf := `
provider "aws" {}
`
	operatorTags := map[string]string{"team": "secops"}
	out := string(applyBuiltinPackParams([]byte(tf), "test.sim", operatorTags))
	if !strings.Contains(out, `variable "default_tags"`) {
		t.Fatalf("expected variable \"default_tags\" block, got:\n%s", out)
	}
	if !strings.Contains(out, "team") || !strings.Contains(out, `"secops"`) {
		t.Errorf("operator tags should appear in default_tags variable default, got:\n%s", out)
	}
	if !strings.Contains(out, "simrun_simulation_id") {
		t.Errorf("simrun_simulation_id should appear in default_tags variable default, got:\n%s", out)
	}
}

func TestApplyBuiltinPackParams_AzureNoTagsResourceSkipped(t *testing.T) {
	tf := `
resource "azurerm_role_assignment" "ra" {
  scope = "/subscriptions/x"
}
`
	out := string(applyBuiltinPackParams([]byte(tf), "test.sim", nil))
	if containsAssignment(out, "tags", "var.default_tags") {
		t.Errorf("azurerm_role_assignment should not get tags, got:\n%s", out)
	}
}

func TestApplyBuiltinPackParams_ParseErrorReturnsOriginal(t *testing.T) {
	tf := []byte(`this is not valid hcl {{{`)
	out := applyBuiltinPackParams(tf, "test.sim", nil)
	if string(out) != string(tf) {
		t.Errorf("expected original content on parse error, got: %s", string(out))
	}
}

// TestApplyBuiltinPackParams_ProducesValidHCL guards against the two ways
// default_tags injection used to emit HCL that fails `terraform init` with
// "Missing newline after block definition":
//   - a source whose final block has no trailing newline left hclwrite
//     without a closing newline token, so the appended variable block ran
//     into the previous brace as `}variable "..." {`;
//   - an empty single-line provider body (`provider "aws" {}`) had no
//     newline token, so the injected nested default_tags block rendered as
//     `provider "aws" { default_tags {`.
func TestApplyBuiltinPackParams_ProducesValidHCL(t *testing.T) {
	tests := []struct {
		name string
		tf   string
	}{
		{
			// No trailing newline after the final output block's closing brace.
			name: "no trailing newline",
			tf: `provider "aws" {
  region = "us-east-1"
}

output "role_arn" {
  value = aws_iam_role.role.arn
}`,
		},
		{
			name: "empty single-line aws provider",
			tf:   "provider \"aws\" {}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := applyBuiltinPackParams([]byte(tt.tf), "test.sim", nil)
			if _, diags := hclwrite.ParseConfig(out, "main.tf", hcl.Pos{Line: 1, Column: 1}); diags.HasErrors() {
				t.Fatalf("output is not valid HCL: %s\n%s", diags, out)
			}
		})
	}
}

func TestRegisterPackParams_ReservedCollision(t *testing.T) {
	resetCustomPackParams()
	defer resetCustomPackParams()
	defer expectPanic(t, "reserved built-in name")
	RegisterPackParams(PackParam{Name: "aws_region", Type: "string"})
}

func TestRegisterPackParams_DuplicateInCall(t *testing.T) {
	resetCustomPackParams()
	defer resetCustomPackParams()
	defer expectPanic(t, "duplicate")
	RegisterPackParams(
		PackParam{Name: "vpc_id", Type: "string"},
		PackParam{Name: "vpc_id", Type: "string"},
	)
}

func TestRegisterPackParams_DuplicateAcrossCalls(t *testing.T) {
	resetCustomPackParams()
	defer resetCustomPackParams()
	RegisterPackParams(PackParam{Name: "vpc_id", Type: "string"})
	defer expectPanic(t, "already registered")
	RegisterPackParams(PackParam{Name: "vpc_id", Type: "string"})
}

func TestRegisterPackParams_DefaultTypeMismatch(t *testing.T) {
	resetCustomPackParams()
	defer resetCustomPackParams()
	defer expectPanic(t, "default-vs-type mismatch")
	RegisterPackParams(PackParam{Name: "enabled", Type: "boolean", Default: "yes"})
}

func TestRegisterPackParams_EnumOnNonString(t *testing.T) {
	resetCustomPackParams()
	defer resetCustomPackParams()
	defer expectPanic(t, "enum is only valid for string")
	RegisterPackParams(PackParam{Name: "enabled", Type: "boolean", Enum: []string{"true"}})
}

func TestRegisterPackParams_UnknownType(t *testing.T) {
	resetCustomPackParams()
	defer resetCustomPackParams()
	defer expectPanic(t, "unknown Type")
	RegisterPackParams(PackParam{Name: "x", Type: "integer"})
}

func TestRegisterPackParams_HappyPath(t *testing.T) {
	resetCustomPackParams()
	defer resetCustomPackParams()
	RegisterPackParams(
		PackParam{Name: "vpc_id", Type: "string", Required: true},
		PackParam{Name: "prefix", Type: "string", Default: "asp"},
	)
	if len(customPackParams) != 2 {
		t.Fatalf("expected 2 custom params, got %d", len(customPackParams))
	}
}

func TestBuildPackParamsSchema_MergesBuiltinsAndCustoms(t *testing.T) {
	resetCustomPackParams()
	defer resetCustomPackParams()
	RegisterPackParams(PackParam{Name: "vpc_id", Type: "string", Required: true})

	schema := buildPackParamsSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", schema["properties"])
	}
	// gcp_project is intentionally NOT a built-in — projects are
	// org-specific, not a knob that benefits from SDK defaults.
	for _, want := range []string{"default_tags", "aws_region", "gcp_region", "azure_location", "vpc_id"} {
		if _, ok := props[want]; !ok {
			t.Errorf("missing property %q in schema", want)
		}
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required slice, got %T", schema["required"])
	}
	found := false
	for _, n := range required {
		if n == "vpc_id" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected vpc_id in required, got %v", required)
	}

	// aws_region, gcp_region, azure_location all have enums.
	for _, name := range []string{"aws_region", "gcp_region", "azure_location"} {
		p, _ := props[name].(map[string]any)
		if _, hasEnum := p["enum"]; !hasEnum {
			t.Errorf("expected %s to have enum", name)
		}
	}
	// default_tags is object with additionalProperties
	dtProp, _ := props["default_tags"].(map[string]any)
	if dtProp["type"] != "object" {
		t.Errorf("default_tags type should be object, got %v", dtProp["type"])
	}
	if _, ok := dtProp["additionalProperties"]; !ok {
		t.Errorf("default_tags should have additionalProperties")
	}
}

func expectPanic(t *testing.T, want string) {
	t.Helper()
	r := recover()
	if r == nil {
		t.Fatalf("expected panic containing %q", want)
	}
	msg, _ := r.(string)
	if !strings.Contains(msg, want) {
		t.Errorf("panic message %q does not contain %q", msg, want)
	}
}
