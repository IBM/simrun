package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestDeriveCloudType(t *testing.T) {
	tests := []struct {
		input    string
		expected CloudType
		wantErr  bool
	}{
		{"aws", CloudTypeEKS, false},
		{"gcp", CloudTypeGKE, false},
		{"azure", CloudTypeAKS, false},
		{"elastic", "", true},
		{"kubernetes", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := DeriveCloudType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeriveCloudType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("DeriveCloudType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestKubeconfigPath(t *testing.T) {
	path := KubeconfigPath("my-cluster")
	expected := filepath.Join(os.TempDir(), "simrun-kubeconfig-my-cluster.yaml")
	if path != expected {
		t.Errorf("KubeconfigPath = %q, want %q", path, expected)
	}
}

func TestParseKubeconfig(t *testing.T) {
	kc := &kubeconfigFile{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []kubeconfigCluster{{
			Name: "test-cluster",
			Cluster: kubeconfigClusterData{
				Server:                   "https://example.com",
				CertificateAuthorityData: "Y2E=",
			},
		}},
		Users: []kubeconfigUser{{
			Name: "test-cluster",
			User: kubeconfigUserData{Token: "mytoken"},
		}},
		Contexts: []kubeconfigContext{{
			Name: "test-cluster",
			Context: kubeconfigContextData{
				Cluster: "test-cluster",
				User:    "test-cluster",
			},
		}},
		CurrentContext: "test-cluster",
	}

	data, err := yaml.Marshal(kc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "kubeconfig-test-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	tmpFile.Close()

	parsed, err := parseKubeconfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("parseKubeconfig failed: %v", err)
	}

	if parsed.CurrentContext != "test-cluster" {
		t.Errorf("expected current-context test-cluster, got %s", parsed.CurrentContext)
	}
	if len(parsed.Clusters) != 1 || parsed.Clusters[0].Cluster.Server != "https://example.com" {
		t.Errorf("unexpected cluster data")
	}
	if len(parsed.Users) != 1 || parsed.Users[0].User.Token != "mytoken" {
		t.Errorf("unexpected user data")
	}
}

func TestParseKubeconfig_NotFound(t *testing.T) {
	_, err := parseKubeconfig("/nonexistent/path/kubeconfig.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestGenerateEKS_MissingCloudType(t *testing.T) {
	_, err := Generate(t.Context(), GenerateConfig{
		ConnectorName: "test",
		CloudType:     "unknown",
		ClusterName:   "cluster",
		Region:        "us-east-1",
	})
	if err == nil {
		t.Error("expected error for unsupported cloud type")
	}
}

func TestGenerateGKE_MissingProject(t *testing.T) {
	_, err := Generate(t.Context(), GenerateConfig{
		ConnectorName: "test",
		CloudType:     CloudTypeGKE,
		ClusterName:   "cluster",
		Region:        "us-central1",
		Project:       "",
	})
	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestGenerateAKS_MissingResourceGroup(t *testing.T) {
	_, err := Generate(t.Context(), GenerateConfig{
		ConnectorName: "test",
		CloudType:     CloudTypeAKS,
		ClusterName:   "cluster",
		Region:        "eastus",
		ResourceGroup: "",
	})
	if err == nil {
		t.Error("expected error for missing resource group")
	}
}
