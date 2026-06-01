package pack

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// extractDeclaredOutputs parses HCL and returns the labels of all top-level
// `output "<name>" {}` blocks. An empty body returns (nil, nil). Parser
// diagnostics are wrapped with the source identifier so callers can produce
// a descriptive panic.
func extractDeclaredOutputs(hclBody string, source string) ([]string, error) {
	if hclBody == "" {
		return nil, nil
	}

	file, diags := hclsyntax.ParseConfig([]byte(hclBody), source, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse %s: %s", source, diags.Error())
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("parse %s: unexpected body type %T", source, file.Body)
	}

	var declared []string
	for _, block := range body.Blocks {
		if block.Type != "output" {
			continue
		}
		if len(block.Labels) != 1 {
			continue
		}
		declared = append(declared, block.Labels[0])
	}
	return declared, nil
}
