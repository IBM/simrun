package pack

import (
	"reflect"
	"testing"
)

func TestExtractDeclaredOutputs(t *testing.T) {
	tests := []struct {
		name    string
		hcl     string
		want    []string
		wantErr bool
	}{
		{
			name: "empty input",
			hcl:  "",
			want: nil,
		},
		{
			name: "single output",
			hcl: `
output "bucket_arn" {
  value = aws_s3_bucket.x.arn
}
`,
			want: []string{"bucket_arn"},
		},
		{
			name: "multiple outputs",
			hcl: `
output "a" { value = "1" }
output "b" { value = "2" }
output "c" { value = "3" }
`,
			want: []string{"a", "b", "c"},
		},
		{
			name: "commented-out output is ignored",
			hcl: `
# output "ghost" { value = "no" }
output "real" { value = "yes" }
`,
			want: []string{"real"},
		},
		{
			name: "heredoc string containing word output is ignored",
			hcl: `
locals {
  doc = <<EOT
output "should_not_match" { value = 1 }
EOT
}

output "real" { value = "yes" }
`,
			want: []string{"real"},
		},
		{
			name: "non-output blocks are ignored",
			hcl: `
resource "aws_s3_bucket" "x" {
  bucket = "x"
}

variable "y" { type = string }

output "z" { value = "z" }
`,
			want: []string{"z"},
		},
		{
			name: "invalid HCL returns error",
			hcl: `
output "broken {
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractDeclaredOutputs(tt.hcl, "test.tf")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractDeclaredOutputs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractDeclaredOutputs_ErrorIncludesSource(t *testing.T) {
	_, err := extractDeclaredOutputs("output \"bad\" {\n", "my-sim.tf")
	if err == nil {
		t.Fatal("expected error")
	}
	if !contains(err.Error(), "my-sim.tf") {
		t.Errorf("error %q does not include source identifier", err.Error())
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
