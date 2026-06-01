package pack

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/hclwrite"
)

// builtinParam describes a single SDK-owned pack-level parameter.
// The rewrite hook is invoked at manifest build time for every sim to
// ensure the matching provider/resource block references var.<name>.
// Returns true if any block was rewritten (so a `variable {}` block is
// also inserted); false means the sim doesn't use the matching cloud
// and the built-in is a no-op for it.
type builtinParam struct {
	param PackParam
	// hclType is the Terraform type expression written into the
	// auto-inserted `variable "<name>" { type = <hclType> }` block.
	hclType string
	// rewrite injects/updates the matching provider/resource block to
	// reference var.<param.Name>. Returns true if a rewrite happened.
	rewrite func(simulationID string, defaultTagsValue map[string]string, file *hclwrite.File) bool
}

// awsRegions is the canonical set of AWS regions exposed in the
// aws_region enum.
var awsRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"ca-central-1",
	"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
	"ap-south-1",
	"ap-southeast-1", "ap-southeast-2",
	"ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
	"sa-east-1",
}

// gcpRegions is the canonical set of GCP regions exposed in the
// gcp_region enum.
var gcpRegions = []string{
	"us-central1", "us-east1", "us-east4", "us-east5", "us-south1",
	"us-west1", "us-west2", "us-west3", "us-west4",
	"northamerica-northeast1", "northamerica-northeast2",
	"southamerica-east1", "southamerica-west1",
	"europe-central2", "europe-north1", "europe-southwest1",
	"europe-west1", "europe-west2", "europe-west3", "europe-west4",
	"europe-west6", "europe-west8", "europe-west9", "europe-west12",
	"asia-east1", "asia-east2",
	"asia-northeast1", "asia-northeast2", "asia-northeast3",
	"asia-south1", "asia-south2",
	"asia-southeast1", "asia-southeast2",
	"australia-southeast1", "australia-southeast2",
	"me-central1", "me-west1",
}

// azureLocations is the canonical set of Azure locations exposed in the
// azure_location enum.
var azureLocations = []string{
	"eastus", "eastus2", "westus", "westus2", "westus3",
	"centralus", "northcentralus", "southcentralus",
	"northeurope", "westeurope", "uksouth", "ukwest",
	"japaneast", "japanwest", "australiaeast", "australiasoutheast",
	"southeastasia", "eastasia",
}

// builtinParams is the ordered, SDK-owned registry of built-in pack
// parameters. Order matters for deterministic schema emission and
// rewriting.
var builtinParams = []builtinParam{
	{
		param: PackParam{
			Name:        "default_tags",
			Type:        PackParamTypeObjectStringMap,
			Description: "Default tags/labels merged into every cloud resource the pack creates.",
			Default:     map[string]string{},
		},
		hclType: "map(string)",
		rewrite: rewriteDefaultTags,
	},
	{
		param: PackParam{
			Name:        "aws_region",
			Type:        PackParamTypeString,
			Description: "AWS region used by the pack's AWS provider.",
			Default:     "us-east-1",
			Enum:        awsRegions,
		},
		hclType: "string",
		rewrite: rewriteAWSRegion,
	},
	{
		param: PackParam{
			Name:        "gcp_region",
			Type:        PackParamTypeString,
			Description: "GCP region used by the pack's google provider.",
			Default:     "us-central1",
			Enum:        gcpRegions,
		},
		hclType: "string",
		rewrite: rewriteGCPRegion,
	},
	{
		param: PackParam{
			Name:        "azure_location",
			Type:        PackParamTypeString,
			Description: "Azure location applied to every azurerm_resource_group.",
			Default:     "eastus",
			Enum:        azureLocations,
		},
		hclType: "string",
		rewrite: rewriteAzureLocation,
	},
}

// reservedBuiltinNames returns the set of param names reserved by the
// built-in registry. Custom RegisterPackParams calls cannot use these.
func reservedBuiltinNames() map[string]struct{} {
	out := make(map[string]struct{}, len(builtinParams))
	for _, b := range builtinParams {
		out[b.param.Name] = struct{}{}
	}
	return out
}

// customPackParams holds parameters registered via RegisterPackParams.
var customPackParams []PackParam

// RegisterPackParams declares one or more custom pack-level parameters.
// Pack authors call this from main() near pack.SetPackInfo. The function
// validates each PackParam synchronously and panics on author bugs
// (reserved-name collision, duplicate name, default-vs-type mismatch,
// enum-on-non-string).
func RegisterPackParams(params ...PackParam) {
	if len(params) == 0 {
		return
	}

	reserved := reservedBuiltinNames()
	existing := make(map[string]struct{}, len(customPackParams))
	for _, p := range customPackParams {
		existing[p.Name] = struct{}{}
	}

	seen := make(map[string]struct{}, len(params))
	for _, p := range params {
		if p.Name == "" {
			panic("RegisterPackParams: PackParam.Name must not be empty")
		}
		if _, isReserved := reserved[p.Name]; isReserved {
			panic(fmt.Sprintf("RegisterPackParams: %q is a reserved built-in name", p.Name))
		}
		if _, dup := seen[p.Name]; dup {
			panic(fmt.Sprintf("RegisterPackParams: duplicate param name %q in call", p.Name))
		}
		if _, dup := existing[p.Name]; dup {
			panic(fmt.Sprintf("RegisterPackParams: param %q already registered", p.Name))
		}
		switch p.Type {
		case PackParamTypeString, PackParamTypeBoolean, PackParamTypeObjectStringMap:
		default:
			panic(fmt.Sprintf("RegisterPackParams: param %q has unknown Type %q", p.Name, p.Type))
		}
		if err := validateDefault(p); err != nil {
			panic(fmt.Sprintf("RegisterPackParams: param %q default-vs-type mismatch: %s", p.Name, err))
		}
		if len(p.Enum) > 0 && p.Type != PackParamTypeString {
			panic(fmt.Sprintf("RegisterPackParams: param %q has Enum but Type is %q (enum is only valid for string)", p.Name, p.Type))
		}
		seen[p.Name] = struct{}{}
	}

	customPackParams = append(customPackParams, params...)
}

// validateDefault checks that p.Default's Go type matches p.Type.
// A nil Default is always acceptable (means "no default").
func validateDefault(p PackParam) error {
	if p.Default == nil {
		return nil
	}
	switch p.Type {
	case PackParamTypeString:
		if _, ok := p.Default.(string); !ok {
			return fmt.Errorf("expected string default, got %T", p.Default)
		}
	case PackParamTypeBoolean:
		if _, ok := p.Default.(bool); !ok {
			return fmt.Errorf("expected bool default, got %T", p.Default)
		}
	case PackParamTypeObjectStringMap:
		switch p.Default.(type) {
		case map[string]string:
		case map[string]any:
		default:
			return fmt.Errorf("expected map[string]string default, got %T", p.Default)
		}
	}
	return nil
}

// resetCustomPackParams clears the custom-param slice. Test-only helper.
func resetCustomPackParams() {
	customPackParams = nil
}
