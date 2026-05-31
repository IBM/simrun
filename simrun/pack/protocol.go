package pack

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// ManifestResponse is the response from the manifest command.
type ManifestResponse struct {
	Pack         PackInfo             `json:"pack"`
	Simulations  []SimulationManifest `json:"simulations"`
	Templates    []TemplateManifest   `json:"templates,omitempty"`
	ParamsSchema json.RawMessage      `json:"params_schema,omitempty"`
}

// PackInfo contains pack metadata.
type PackInfo struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	MinSimrunVersion string `json:"min_simrun_version"`
}

// ManifestInput is the input to the manifest command, read from stdin.
// Parameters are optional key-value configuration provided by simrun.
// The "default_tags" key is treated specially: its value (a string map) is
// injected as default tags/labels into all simulations' Terraform.
type ManifestInput struct {
	Parameters map[string]any `json:"parameters,omitempty"`
}

// SimulationManifest is the manifest entry for a single simulation.
type SimulationManifest struct {
	ID                        string          `json:"id"` // Simulation ID: scope.slug (e.g., "aws.ec2-bitcoin-mining")
	Name                      string          `json:"name"`
	Description               string          `json:"description"`
	MITRE                     MITREMapping    `json:"mitre"`
	Scope                     string          `json:"scope"`
	IsSlow                    bool            `json:"is_slow,omitempty"`
	RequiresExternalResources bool            `json:"requires_external_resources,omitempty"`
	ParamsSchema              json.RawMessage `json:"params_schema,omitempty"`
	Terraform                 string          `json:"terraform,omitempty"` // base64-encoded
	HasCustomCleanup          bool            `json:"has_custom_cleanup"`
}

// TemplateManifest is the manifest entry for an injection template.
type TemplateManifest struct {
	ID          string            `json:"id"`          // Template ID: scope.slug (e.g., "okta.add-group-member")
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Scope       string            `json:"scope"`
	Content     string            `json:"content"`         // Base64-encoded template content
	Vars        map[string]string `json:"vars,omitempty"`  // Variable names to default values, extracted from template
}

// LogLine represents a JSON log line written to stderr by a pack.
type LogLine struct {
	Level       string         `json:"level"`
	Msg         string         `json:"msg"`
	Simulation  string         `json:"simulation,omitempty"`
	ExecutionID string         `json:"execution_id,omitempty"`
	Pack        string         `json:"pack,omitempty"`
	PackVersion string         `json:"pack_version,omitempty"`
	Timestamp   string         `json:"ts"`
	Extra       map[string]any `json:"-"` // Additional fields not in the struct
}

// UnmarshalJSON implements custom unmarshaling to capture extra fields.
func (l *LogLine) UnmarshalJSON(data []byte) error {
	type Alias LogLine
	if err := json.Unmarshal(data, (*Alias)(l)); err != nil {
		return err
	}

	extra, err := extractExtraFields(data, []string{
		"level", "msg", "simulation", "execution_id", "pack", "pack_version", "ts",
	})
	if err != nil {
		return err
	}
	l.Extra = extra
	return nil
}

// Log levels.
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// extractExtraFields unmarshals JSON data and returns fields not in knownFields list.
func extractExtraFields(data []byte, knownFields []string) (map[string]any, error) {
	var allFields map[string]any
	if err := json.Unmarshal(data, &allFields); err != nil {
		return nil, err
	}

	for _, field := range knownFields {
		delete(allFields, field)
	}

	if len(allFields) == 0 {
		return nil, nil
	}
	return allFields, nil
}

// buildManifest creates a ManifestResponse from all registered simulations.
// It injects default tags into each simulation's Terraform content and
// base64-encodes it for the manifest protocol.
func buildManifest(defaultTags map[string]string) ManifestResponse {
	simulations := make([]SimulationManifest, 0, len(registry))
	for id, s := range registry {
		var paramsSchema json.RawMessage
		if s.ParamsSchema != nil {
			paramsSchema, _ = json.Marshal(s.ParamsSchema)
		} else if s.Terraform != "" {
			if schema := extractTerraformVarsSchema(s.Terraform); schema != nil {
				paramsSchema, _ = json.Marshal(schema)
			}
		}

		var terraform string
		if s.Terraform != "" {
			tfBytes := applyBuiltinPackParams([]byte(s.Terraform), id, defaultTags)
			terraform = base64.StdEncoding.EncodeToString(tfBytes)
		}

		simulations = append(simulations, SimulationManifest{
			ID:                        id,
			Name:                      s.Name,
			Description:               s.Description,
			MITRE:                     s.MITRE,
			Scope:                     s.Scope,
			IsSlow:                    s.IsSlow,
			RequiresExternalResources: s.RequiresExternalResources,
			ParamsSchema:              paramsSchema,
			Terraform:                 terraform,
			HasCustomCleanup:          s.Cleanup != nil,
		})
	}

	// Build templates
	templates := make([]TemplateManifest, 0, len(templateRegistry))
	for id, t := range templateRegistry {
		templates = append(templates, TemplateManifest{
			ID:          id,
			Name:        t.Name,
			Description: t.Description,
			Scope:       t.Scope,
			Content:     base64.StdEncoding.EncodeToString([]byte(t.Content)),
			Vars:        extractTemplateVars(t.Content),
		})
	}

	resp := ManifestResponse{
		Pack: PackInfo{
			Name:             packName,
			Version:          packVersion,
			MinSimrunVersion: minSimrunVersion,
		},
		Simulations: simulations,
		Templates:   templates,
	}

	if schema := buildPackParamsSchema(); schema != nil {
		raw, err := json.Marshal(schema)
		if err == nil {
			resp.ParamsSchema = raw
		}
	}

	return resp
}

// buildPackParamsSchema merges the built-in registry with author-declared
// custom params into a JSON-Schema-shaped document of the form:
//
//	{"properties": {"<name>": {...}, ...}, "required": ["<name>", ...]}
//
// String→string maps are expressed as
// `{"type": "object", "additionalProperties": {"type": "string"}}`.
// Returns nil when both the built-in registry and the custom-param slice
// are empty so the caller can omit the manifest field.
func buildPackParamsSchema() map[string]any {
	if len(builtinParams) == 0 && len(customPackParams) == 0 {
		return nil
	}

	properties := make(map[string]any)
	var required []string

	addParam := func(p PackParam) {
		prop := map[string]any{}
		switch p.Type {
		case PackParamTypeString:
			prop["type"] = "string"
			if len(p.Enum) > 0 {
				prop["enum"] = p.Enum
			}
		case PackParamTypeBoolean:
			prop["type"] = "boolean"
		case PackParamTypeObjectStringMap:
			prop["type"] = "object"
			prop["additionalProperties"] = map[string]any{"type": "string"}
		}
		if p.Description != "" {
			prop["description"] = p.Description
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}
		properties[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}

	for _, b := range builtinParams {
		addParam(b.param)
	}
	for _, p := range customPackParams {
		addParam(p)
	}

	out := map[string]any{"properties": properties}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

// extractTerraformVarsSchema parses Terraform HCL content and extracts
// variable blocks into a JSON Schema-compatible map. This is used as a
// fallback when a simulation doesn't explicitly set ParamsSchema but has
// Terraform variables that users can configure via params.
//
// The returned schema has the format:
//
//	{"properties": {"var_name": {"type": "string", "description": "..."}}}
//
// Returns nil if no variables are found or the HCL cannot be parsed.
func extractTerraformVarsSchema(tfContent string) map[string]any {
	f, diags := hclwrite.ParseConfig([]byte(tfContent), "main.tf", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil
	}

	properties := make(map[string]any)
	for _, block := range f.Body().Blocks() {
		if block.Type() != "variable" {
			continue
		}
		labels := block.Labels()
		if len(labels) == 0 {
			continue
		}
		varName := labels[0]

		prop := map[string]any{
			"type": "string",
		}

		if desc := extractStringAttribute(block, "description"); desc != "" {
			prop["description"] = desc
		}

		if def := extractStringAttribute(block, "default"); def != "" {
			prop["default"] = def
		}

		properties[varName] = prop
	}

	if len(properties) == 0 {
		return nil
	}

	return map[string]any{
		"properties": properties,
	}
}

// templateVarPattern matches Go template variable references:
//   - {{ .VarName }}
//   - {{ or .VarName "default" }}
var templateVarPattern = regexp.MustCompile(`\{\{\s*(?:or\s+)?\.(\w+)(?:\s+"([^"]*)")?\s*\}\}`)

// extractTemplateVars parses Go template content and extracts user-configurable
// variables with their default values. Built-in variables (Timestamp, ExecutionID)
// are filtered out. Returns nil if no user variables are found.
func extractTemplateVars(content string) map[string]string {
	if content == "" {
		return nil
	}

	builtins := map[string]bool{
		"Timestamp":   true,
		"ExecutionID": true,
	}

	vars := make(map[string]string)
	for _, match := range templateVarPattern.FindAllStringSubmatch(content, -1) {
		name := match[1]
		if builtins[name] {
			continue
		}
		if _, exists := vars[name]; exists {
			continue
		}
		defaultVal := ""
		if len(match) > 2 {
			defaultVal = match[2]
		}
		vars[name] = defaultVal
	}

	if len(vars) == 0 {
		return nil
	}
	return vars
}

// extractStringAttribute reads a string attribute value from an HCL block.
// It parses the attribute's expression tokens to extract a quoted string literal.
// Returns empty string if the attribute doesn't exist or isn't a simple string.
func extractStringAttribute(block *hclwrite.Block, name string) string {
	attr := block.Body().GetAttribute(name)
	if attr == nil {
		return ""
	}

	// Extract string value from expression tokens.
	// hclwrite tokens for a string literal look like: TokenOQuote "value" TokenCQuote
	tokens := attr.Expr().BuildTokens(nil)
	var parts []string
	for _, t := range tokens {
		if t.Type == hclsyntax.TokenQuotedLit {
			parts = append(parts, string(t.Bytes))
		}
	}
	return strings.Join(parts, "")
}
