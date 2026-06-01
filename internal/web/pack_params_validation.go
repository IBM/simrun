package web

import (
	"encoding/json"
	"fmt"
)

// paramSchema is a minimal projection of the JSON-Schema doc that lives
// in pack.ManifestResponse.ParamsSchema. We only care about the fields
// needed for PUT-time validation.
type paramSchema struct {
	Properties map[string]paramProperty `json:"properties"`
	Required   []string                 `json:"required"`
}

type paramProperty struct {
	Type                 string         `json:"type"`
	Enum                 []string       `json:"enum"`
	AdditionalProperties map[string]any `json:"additionalProperties"`
}

// paramValidationError is one row in the validation error response.
type paramValidationError struct {
	Key  string `json:"key"`
	Rule string `json:"rule"`
	Msg  string `json:"message"`
}

// validatePackParameters strict-validates declared keys in `values`
// against `schema` and returns:
//   - errors: per-key validation failures, empty when all declared keys
//     pass.
//   - unknownKeys: keys present in `values` but absent from
//     schema.Properties.
//
// The caller should reject the request if errors is non-empty and treat
// unknownKeys as a soft warning otherwise.
func validatePackParameters(schema *paramSchema, values map[string]any) (errors []paramValidationError, unknownKeys []string) {
	if schema == nil || len(schema.Properties) == 0 {
		// Permissive: every key is unknown, no errors raised.
		for k := range values {
			unknownKeys = append(unknownKeys, k)
		}
		return nil, unknownKeys
	}

	// Required-key check: schema declares required, body must include it.
	for _, name := range schema.Required {
		if _, ok := values[name]; !ok {
			errors = append(errors, paramValidationError{
				Key:  name,
				Rule: "required",
				Msg:  fmt.Sprintf("missing required parameter %q", name),
			})
		}
	}

	for key, raw := range values {
		prop, declared := schema.Properties[key]
		if !declared {
			unknownKeys = append(unknownKeys, key)
			continue
		}
		if err := validateValue(key, prop, raw); err != nil {
			errors = append(errors, *err)
		}
	}

	return errors, unknownKeys
}

// validateValue checks one declared key's value against its property
// spec. Returns nil on success.
func validateValue(key string, prop paramProperty, raw any) *paramValidationError {
	switch prop.Type {
	case "string":
		s, ok := raw.(string)
		if !ok {
			return &paramValidationError{
				Key:  key,
				Rule: "type",
				Msg:  fmt.Sprintf("expected string, got %T", raw),
			}
		}
		if len(prop.Enum) > 0 {
			for _, allowed := range prop.Enum {
				if s == allowed {
					return nil
				}
			}
			return &paramValidationError{
				Key:  key,
				Rule: "enum",
				Msg:  fmt.Sprintf("value %q is not one of allowed values %v", s, prop.Enum),
			}
		}
	case "boolean":
		if _, ok := raw.(bool); !ok {
			return &paramValidationError{
				Key:  key,
				Rule: "type",
				Msg:  fmt.Sprintf("expected boolean, got %T", raw),
			}
		}
	case "object":
		obj, ok := raw.(map[string]any)
		if !ok {
			return &paramValidationError{
				Key:  key,
				Rule: "type",
				Msg:  fmt.Sprintf("expected object, got %T", raw),
			}
		}
		// String→string map case (additionalProperties.type == "string").
		if ap := prop.AdditionalProperties; ap != nil {
			if t, _ := ap["type"].(string); t == "string" {
				for k, v := range obj {
					if _, ok := v.(string); !ok {
						return &paramValidationError{
							Key:  key,
							Rule: "type",
							Msg:  fmt.Sprintf("expected string value for %q.%q, got %T", key, k, v),
						}
					}
				}
			}
		}
	}
	return nil
}

// parsePackParamsSchema unmarshals the raw json.RawMessage from
// ManifestResponse.ParamsSchema into the projection we use for
// validation. Returns nil when the input is empty.
func parsePackParamsSchema(raw json.RawMessage) (*paramSchema, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var s paramSchema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
