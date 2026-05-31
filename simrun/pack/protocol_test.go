package pack

import (
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type wantVar struct {
	description string
	defaultVal  string
}

func TestExtractTerraformVarsSchema(t *testing.T) {
	tests := []struct {
		name     string
		tfContent string
		wantNil  bool
		wantVars map[string]wantVar
	}{
		{
			name: "single variable with description and default",
			tfContent: `
variable "cluster_name" {
  description = "Name of the EKS cluster to target"
  type        = string
  default     = "my-cluster"
}
`,
			wantVars: map[string]wantVar{
				"cluster_name": {description: "Name of the EKS cluster to target", defaultVal: "my-cluster"},
			},
		},
		{
			name: "multiple variables",
			tfContent: `
variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "count" {
  type = number
}
`,
			wantVars: map[string]wantVar{
				"cluster_name": {description: "Name of the EKS cluster"},
				"region":       {description: "AWS region", defaultVal: "us-east-1"},
				"count":        {},
			},
		},
		{
			name: "no variables",
			tfContent: `
provider "aws" {
  region = "us-east-1"
}

resource "aws_instance" "this" {
  ami = "ami-123"
}
`,
			wantNil: true,
		},
		{
			name:    "empty content",
			wantNil: true,
		},
		{
			name:      "invalid HCL",
			tfContent: `this is not valid HCL {{{}`,
			wantNil:   true,
		},
		{
			name: "mixed blocks with variables",
			tfContent: `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "cluster_name" {
  description = "Name of the EKS cluster to target"
  type        = string
  default     = "k8saas-k8s-rhvx6"
}

provider "aws" {
  skip_region_validation = true
}

output "binding_name" {
  description = "Name of the created ClusterRoleBinding"
  value       = kubernetes_cluster_role_binding.this.metadata[0].name
}
`,
			wantVars: map[string]wantVar{
				"cluster_name": {description: "Name of the EKS cluster to target", defaultVal: "k8saas-k8s-rhvx6"},
			},
		},
		{
			name: "variable without description or default",
			tfContent: `
variable "bucket_name" {
  type = string
}
`,
			wantVars: map[string]wantVar{
				"bucket_name": {},
			},
		},
		{
			name: "variable with default only",
			tfContent: `
variable "bucket_name" {
  type    = string
  default = "my-bucket"
}
`,
			wantVars: map[string]wantVar{
				"bucket_name": {defaultVal: "my-bucket"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTerraformVarsSchema(tt.tfContent)

			if tt.wantNil {
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			properties, ok := result["properties"].(map[string]any)
			if !ok {
				t.Fatalf("expected properties to be map[string]any, got %T", result["properties"])
			}

			if len(properties) != len(tt.wantVars) {
				t.Fatalf("expected %d properties, got %d: %v", len(tt.wantVars), len(properties), properties)
			}

			for varName, want := range tt.wantVars {
				prop, ok := properties[varName]
				if !ok {
					t.Errorf("missing property %q", varName)
					continue
				}

				propMap, ok := prop.(map[string]any)
				if !ok {
					t.Errorf("property %q: expected map[string]any, got %T", varName, prop)
					continue
				}

				if propMap["type"] != "string" {
					t.Errorf("property %q: expected type 'string', got %q", varName, propMap["type"])
				}

				gotDesc, _ := propMap["description"].(string)
				if gotDesc != want.description {
					t.Errorf("property %q: expected description %q, got %q", varName, want.description, gotDesc)
				}
				if want.description == "" {
					if _, has := propMap["description"]; has {
						t.Errorf("property %q: expected no description key, but it exists", varName)
					}
				}

				gotDefault, _ := propMap["default"].(string)
				if gotDefault != want.defaultVal {
					t.Errorf("property %q: expected default %q, got %q", varName, want.defaultVal, gotDefault)
				}
				if want.defaultVal == "" {
					if _, has := propMap["default"]; has {
						t.Errorf("property %q: expected no default key, but it exists", varName)
					}
				}
			}
		})
	}
}

func TestExtractStringAttribute(t *testing.T) {
	tests := []struct {
		name string
		hcl  string
		attr string
		want string
	}{
		{
			name: "simple string",
			hcl: `variable "x" {
  description = "hello world"
}`,
			attr: "description",
			want: "hello world",
		},
		{
			name: "missing attribute",
			hcl: `variable "x" {
  type = string
}`,
			attr: "description",
			want: "",
		},
		{
			name: "default value",
			hcl: `variable "x" {
  default = "my-value"
}`,
			attr: "default",
			want: "my-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, diags := hclwrite.ParseConfig([]byte(tt.hcl), "test.tf", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatalf("failed to parse HCL: %s", diags.Error())
			}

			blocks := f.Body().Blocks()
			if len(blocks) == 0 {
				t.Fatal("no blocks found")
			}

			got := extractStringAttribute(blocks[0], tt.attr)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestExtractTemplateVars(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "no variables",
			content: `{"message": "hello world"}`,
			want:    nil,
		},
		{
			name:    "only built-in variables",
			content: `{"timestamp": "{{ .Timestamp }}", "uuid": "{{ .ExecutionID }}"}`,
			want:    nil,
		},
		{
			name:    "simple variable without default",
			content: `{"actor": "{{ .ActorEmail }}"}`,
			want:    map[string]string{"ActorEmail": ""},
		},
		{
			name:    "variable with default using or",
			content: `{"id": "{{ or .TargetGroupID "00grbtp7hjaftSHcj356" }}"}`,
			want:    map[string]string{"TargetGroupID": "00grbtp7hjaftSHcj356"},
		},
		{
			name: "multiple variables mixed with built-ins",
			content: `{
				"@timestamp": "{{ .Timestamp }}",
				"actor": "{{ or .ActorEmail "admin@test.com" }}",
				"display": "{{ or .ActorDisplayName "Admin" }}",
				"uuid": "{{ .ExecutionID }}",
				"target": "{{ or .TargetGroupID "00g123" }}"
			}`,
			want: map[string]string{
				"ActorEmail":       "admin@test.com",
				"ActorDisplayName": "Admin",
				"TargetGroupID":    "00g123",
			},
		},
		{
			name:    "duplicate variable uses first occurrence",
			content: `{"a": "{{ or .ActorEmail "first" }}", "b": "{{ .ActorEmail }}"}`,
			want:    map[string]string{"ActorEmail": "first"},
		},
		{
			name:    "variable with empty default",
			content: `{"name": "{{ or .UserName "" }}"}`,
			want:    map[string]string{"UserName": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTemplateVars(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractTemplateVars() = %v, want %v", got, tt.want)
			}
		})
	}
}
