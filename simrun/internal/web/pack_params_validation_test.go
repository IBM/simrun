package web

import (
	"encoding/json"
	"testing"
)

func TestValidatePackParameters_PermissiveWhenSchemaEmpty(t *testing.T) {
	errs, unknown := validatePackParameters(nil, map[string]any{"anything": "goes"})
	if len(errs) != 0 {
		t.Errorf("expected no errors with nil schema, got %v", errs)
	}
	if len(unknown) != 1 || unknown[0] != "anything" {
		t.Errorf("expected unknown_keys=[anything], got %v", unknown)
	}
}

func TestValidatePackParameters_TypeMismatch(t *testing.T) {
	schema := mustSchema(t, `{"properties": {"aws_region": {"type": "string"}}}`)
	errs, _ := validatePackParameters(schema, map[string]any{"aws_region": 5})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %v", errs)
	}
	if errs[0].Key != "aws_region" || errs[0].Rule != "type" {
		t.Errorf("expected type error on aws_region, got %+v", errs[0])
	}
}

func TestValidatePackParameters_EnumViolation(t *testing.T) {
	schema := mustSchema(t, `{"properties": {"aws_region": {"type": "string", "enum": ["us-east-1", "us-west-2"]}}}`)
	errs, _ := validatePackParameters(schema, map[string]any{"aws_region": "eu-west-9"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %v", errs)
	}
	if errs[0].Rule != "enum" {
		t.Errorf("expected enum error, got %+v", errs[0])
	}
}

func TestValidatePackParameters_EnumPass(t *testing.T) {
	schema := mustSchema(t, `{"properties": {"aws_region": {"type": "string", "enum": ["us-east-1"]}}}`)
	errs, _ := validatePackParameters(schema, map[string]any{"aws_region": "us-east-1"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidatePackParameters_MissingRequired(t *testing.T) {
	schema := mustSchema(t, `{"properties": {"vpc_id": {"type": "string"}}, "required": ["vpc_id"]}`)
	errs, _ := validatePackParameters(schema, map[string]any{})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %v", errs)
	}
	if errs[0].Rule != "required" || errs[0].Key != "vpc_id" {
		t.Errorf("expected required error on vpc_id, got %+v", errs[0])
	}
}

func TestValidatePackParameters_UnknownKeyPassthrough(t *testing.T) {
	schema := mustSchema(t, `{"properties": {"aws_region": {"type": "string"}}}`)
	errs, unknown := validatePackParameters(schema, map[string]any{
		"aws_region": "us-east-1",
		"legacy_key": "x",
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if len(unknown) != 1 || unknown[0] != "legacy_key" {
		t.Errorf("expected unknown_keys=[legacy_key], got %v", unknown)
	}
}

func TestValidatePackParameters_BooleanType(t *testing.T) {
	schema := mustSchema(t, `{"properties": {"enabled": {"type": "boolean"}}}`)
	errs, _ := validatePackParameters(schema, map[string]any{"enabled": "true"})
	if len(errs) != 1 || errs[0].Rule != "type" {
		t.Errorf("expected type error, got %v", errs)
	}

	errs, _ = validatePackParameters(schema, map[string]any{"enabled": true})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidatePackParameters_ObjectStringMap(t *testing.T) {
	schema := mustSchema(t, `{
		"properties": {
			"default_tags": {"type": "object", "additionalProperties": {"type": "string"}}
		}
	}`)

	// Bad: nested non-string value.
	errs, _ := validatePackParameters(schema, map[string]any{
		"default_tags": map[string]any{"a": 5},
	})
	if len(errs) != 1 || errs[0].Rule != "type" {
		t.Errorf("expected nested type error, got %v", errs)
	}

	// Good: all-string values.
	errs, _ = validatePackParameters(schema, map[string]any{
		"default_tags": map[string]any{"a": "1"},
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}

	// Bad: non-object value.
	errs, _ = validatePackParameters(schema, map[string]any{"default_tags": "string"})
	if len(errs) != 1 || errs[0].Rule != "type" {
		t.Errorf("expected non-object type error, got %v", errs)
	}
}

func mustSchema(t *testing.T, src string) *paramSchema {
	t.Helper()
	s, err := parsePackParamsSchema(json.RawMessage(src))
	if err != nil {
		t.Fatalf("parsePackParamsSchema: %v", err)
	}
	return s
}
