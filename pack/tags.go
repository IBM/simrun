package pack

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// applyBuiltinPackParams rewrites Terraform HCL content to wire built-in
// pack-level params through Terraform variables. For each built-in whose
// matching provider/resource block is present, the function rewrites the
// block to reference var.<name> and ensures a corresponding
// `variable "<name>" {}` block is declared with the schema-derived type
// and default.
//
// defaultTagsValue is the operator-supplied default_tags value (typically
// from ManifestInput.Parameters["default_tags"]); it's baked into the
// default_tags variable's default value along with simrun_simulation_id.
//
// Returns the original content unchanged if it cannot be parsed as HCL.
func applyBuiltinPackParams(tfContent []byte, simulationID string, defaultTagsValue map[string]string) []byte {
	// A source file with no trailing newline leaves hclwrite's final block
	// without a closing newline token, so AppendNewBlock would emit a
	// malformed `}variable "..." {` run. Normalize before parsing.
	normalized := tfContent
	if len(normalized) > 0 && normalized[len(normalized)-1] != '\n' {
		normalized = append(normalized[:len(normalized):len(normalized)], '\n')
	}

	f, diags := hclwrite.ParseConfig(normalized, "main.tf", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return tfContent
	}

	for _, b := range builtinParams {
		if b.rewrite == nil {
			continue
		}
		rewrote := b.rewrite(simulationID, defaultTagsValue, f)
		if rewrote {
			ensureVariableBlock(f, b, simulationID, defaultTagsValue)
		}
	}

	return f.Bytes()
}

// ensureVariableBlock inserts a `variable "<name>" {}` block for the given
// built-in if one is not already present. The block's `type` and `default`
// attributes are derived from the built-in's schema entry. For
// default_tags, the default also carries simrun_simulation_id so the tag
// flows through even without operator-supplied values.
func ensureVariableBlock(f *hclwrite.File, b builtinParam, simulationID string, defaultTagsValue map[string]string) {
	for _, existing := range f.Body().Blocks() {
		if existing.Type() == "variable" && len(existing.Labels()) == 1 && existing.Labels()[0] == b.param.Name {
			return
		}
	}

	block := f.Body().AppendNewBlock("variable", []string{b.param.Name})
	block.Body().SetAttributeRaw("type", hclTypeTokens(b.hclType))

	defaultVal := builtinDefaultValue(b, simulationID, defaultTagsValue)
	if defaultVal != cty.NilVal {
		block.Body().SetAttributeValue("default", defaultVal)
	}
}

// builtinDefaultValue computes the cty value used for a built-in's
// variable default. For default_tags, the operator-supplied tags (if
// any) are merged with simrun_simulation_id. For other built-ins the
// schema default is used as-is.
func builtinDefaultValue(b builtinParam, simulationID string, defaultTagsValue map[string]string) cty.Value {
	switch b.param.Name {
	case "default_tags":
		tags := make(map[string]string, len(defaultTagsValue)+1)
		for k, v := range defaultTagsValue {
			tags[k] = v
		}
		// GCP label values (default_tags flows into google default_labels)
		// forbid dots; simulation IDs are namespaced with them
		// (e.g. "gcp.gcs-list-objects"). Sanitize once here so the same
		// default_tags map is valid as AWS/Azure tags and GCP labels alike.
		tags["simrun_simulation_id"] = strings.ReplaceAll(simulationID, ".", "_")
		return buildCtyMap(tags)
	}
	switch v := b.param.Default.(type) {
	case string:
		if v == "" {
			return cty.NilVal
		}
		return cty.StringVal(v)
	case bool:
		return cty.BoolVal(v)
	case map[string]string:
		if len(v) == 0 {
			return cty.NilVal
		}
		return buildCtyMap(v)
	}
	return cty.NilVal
}

// hclTypeTokens builds tokens for a Terraform variable type expression
// like `string`, `bool`, or `map(string)`.
func hclTypeTokens(hclType string) hclwrite.Tokens {
	// Simple identifier case.
	if !strings.Contains(hclType, "(") {
		return hclwrite.Tokens{
			{Type: hclsyntax.TokenIdent, Bytes: []byte(hclType)},
		}
	}
	// Function-call case (e.g. map(string)).
	open := strings.Index(hclType, "(")
	close := strings.LastIndex(hclType, ")")
	if close <= open {
		return hclwrite.Tokens{{Type: hclsyntax.TokenIdent, Bytes: []byte(hclType)}}
	}
	outer := hclType[:open]
	inner := hclType[open+1 : close]
	return hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(outer)},
		{Type: hclsyntax.TokenOParen, Bytes: []byte("(")},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(inner)},
		{Type: hclsyntax.TokenCParen, Bytes: []byte(")")},
	}
}

// rewriteDefaultTags adds or merges default_tags in every AWS provider
// block, default_labels on every google provider, and tags on every
// azurerm_* resource, in each case using a `merge(var.default_tags,
// existing)` expression so operator-supplied tags layer underneath the
// sim's own tags.
func rewriteDefaultTags(simulationID string, _ map[string]string, f *hclwrite.File) bool {
	rewrote := false
	for _, block := range f.Body().Blocks() {
		switch {
		case block.Type() == "provider" && hasLabel(block, "aws"):
			injectAWSDefaultTagsVar(block)
			rewrote = true
		case block.Type() == "provider" && hasLabel(block, "google"):
			injectGCPDefaultLabelsVar(block)
			rewrote = true
		case block.Type() == "resource" && hasAzureResourceLabel(block):
			if injectAzureResourceTagsVar(block) {
				rewrote = true
			}
		}
	}
	return rewrote
}

// rewriteAWSRegion injects `region = var.aws_region` into every
// `provider "aws" {}` block that does not already declare a region
// attribute. Returns true if at least one provider block was found.
func rewriteAWSRegion(_ string, _ map[string]string, f *hclwrite.File) bool {
	found := false
	for _, block := range f.Body().Blocks() {
		if block.Type() != "provider" || !hasLabel(block, "aws") {
			continue
		}
		found = true
		if block.Body().GetAttribute("region") != nil {
			continue
		}
		block.Body().SetAttributeRaw("region", varRefTokens("aws_region"))
	}
	return found
}

// rewriteGCPRegion injects `region = var.gcp_region` into every
// `provider "google" {}` block missing the attribute.
func rewriteGCPRegion(_ string, _ map[string]string, f *hclwrite.File) bool {
	found := false
	for _, block := range f.Body().Blocks() {
		if block.Type() != "provider" || !hasLabel(block, "google") {
			continue
		}
		found = true
		if block.Body().GetAttribute("region") != nil {
			continue
		}
		block.Body().SetAttributeRaw("region", varRefTokens("gcp_region"))
	}
	return found
}

// rewriteAzureLocation injects `location = var.azure_location` into every
// `resource "azurerm_resource_group" "<x>" {}` block missing the
// attribute. Other azurerm_* resources are left alone.
func rewriteAzureLocation(_ string, _ map[string]string, f *hclwrite.File) bool {
	found := false
	for _, block := range f.Body().Blocks() {
		if block.Type() != "resource" {
			continue
		}
		labels := block.Labels()
		if len(labels) == 0 || labels[0] != "azurerm_resource_group" {
			continue
		}
		found = true
		if block.Body().GetAttribute("location") != nil {
			continue
		}
		block.Body().SetAttributeRaw("location", varRefTokens("azure_location"))
	}
	return found
}

// varRefTokens builds tokens for `var.<name>`.
func varRefTokens(name string) hclwrite.Tokens {
	return hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte("var")},
		{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(name)},
	}
}

// injectAWSDefaultTagsVar rewrites an AWS provider block to declare a
// default_tags block that merges var.default_tags into any existing
// tags map.
func injectAWSDefaultTagsVar(provider *hclwrite.Block) {
	var existingTagsTokens hclwrite.Tokens
	var toRemove []*hclwrite.Block
	for _, b := range provider.Body().Blocks() {
		if b.Type() == "default_tags" {
			if attr := b.Body().GetAttribute("tags"); attr != nil {
				existingTagsTokens = attr.Expr().BuildTokens(nil)
			}
			toRemove = append(toRemove, b)
		}
	}
	for _, b := range toRemove {
		provider.Body().RemoveBlock(b)
	}

	// An empty single-line body (`provider "aws" {}`) has no newline token,
	// so AppendNewBlock would render `provider "aws" { default_tags {` —
	// invalid HCL. Force the body multi-line first.
	if len(provider.Body().Attributes()) == 0 && len(provider.Body().Blocks()) == 0 {
		provider.Body().AppendNewline()
	}

	dt := provider.Body().AppendNewBlock("default_tags", nil)
	if existingTagsTokens != nil {
		dt.Body().SetAttributeRaw("tags", mergeVarTokens("default_tags", existingTagsTokens))
	} else {
		dt.Body().SetAttributeRaw("tags", varRefTokens("default_tags"))
	}
}

// injectGCPDefaultLabelsVar rewrites a google provider block to merge
// var.default_tags into default_labels.
func injectGCPDefaultLabelsVar(provider *hclwrite.Block) {
	existing := provider.Body().GetAttribute("default_labels")
	if existing == nil {
		provider.Body().SetAttributeRaw("default_labels", varRefTokens("default_tags"))
		return
	}
	provider.Body().SetAttributeRaw("default_labels", mergeVarTokens("default_tags", existing.Expr().BuildTokens(nil)))
}

// injectAzureResourceTagsVar rewrites an azurerm_* resource block to
// merge var.default_tags into its tags attribute. Resources that don't
// support tags are skipped (returning false so the caller doesn't insert
// a variable block on their account).
func injectAzureResourceTagsVar(resource *hclwrite.Block) bool {
	labels := resource.Labels()
	if len(labels) > 0 && azureNoTagsResources[labels[0]] {
		return false
	}
	existing := resource.Body().GetAttribute("tags")
	if existing == nil {
		resource.Body().SetAttributeRaw("tags", varRefTokens("default_tags"))
		return true
	}
	resource.Body().SetAttributeRaw("tags", mergeVarTokens("default_tags", existing.Expr().BuildTokens(nil)))
	return true
}

// azureNoTagsResources is a set of Azure resource types that don't support tags.
var azureNoTagsResources = map[string]bool{
	"azurerm_subnet": true,
	"azurerm_subnet_network_security_group_association":          true,
	"azurerm_subnet_route_table_association":                     true,
	"azurerm_subnet_nat_gateway_association":                     true,
	"azurerm_network_interface_security_group_association":       true,
	"azurerm_network_interface_backend_address_pool_association": true,
	"azurerm_role_assignment":                                    true,
	"azurerm_role_definition":                                    true,
	"azurerm_management_lock":                                    true,
	// Storage data-plane resources are scoped to a storage account and do
	// not accept tags.
	"azurerm_storage_container":                 true,
	"azurerm_storage_blob":                      true,
	"azurerm_storage_share":                     true,
	"azurerm_storage_share_directory":           true,
	"azurerm_storage_share_file":                true,
	"azurerm_storage_queue":                     true,
	"azurerm_storage_table":                     true,
	"azurerm_storage_table_entity":              true,
	"azurerm_storage_data_lake_gen2_filesystem": true,
	"azurerm_storage_data_lake_gen2_path":       true,
}

// mergeVarTokens builds tokens for `merge(var.<name>, <existing>)`.
func mergeVarTokens(varName string, existingTokens hclwrite.Tokens) hclwrite.Tokens {
	var tokens hclwrite.Tokens
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte("merge")},
		&hclwrite.Token{Type: hclsyntax.TokenOParen, Bytes: []byte("(")},
	)
	tokens = append(tokens, varRefTokens(varName)...)
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenComma, Bytes: []byte(",")},
		&hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte(" ")},
	)
	tokens = append(tokens, existingTokens...)
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenCParen, Bytes: []byte(")")},
	)
	return tokens
}

// buildCtyMap converts a Go string map to a cty object value for HCL
// generation. An empty map produces cty.EmptyObjectVal.
func buildCtyMap(tags map[string]string) cty.Value {
	if len(tags) == 0 {
		return cty.EmptyObjectVal
	}
	vals := make(map[string]cty.Value, len(tags))
	for k, v := range tags {
		vals[k] = cty.StringVal(v)
	}
	return cty.ObjectVal(vals)
}

// hasLabel checks if a block has the given first label.
func hasLabel(block *hclwrite.Block, label string) bool {
	labels := block.Labels()
	return len(labels) > 0 && labels[0] == label
}

// hasAzureResourceLabel checks if a resource block is an Azure resource type.
func hasAzureResourceLabel(block *hclwrite.Block) bool {
	labels := block.Labels()
	return len(labels) > 0 && strings.HasPrefix(labels[0], "azurerm_")
}
