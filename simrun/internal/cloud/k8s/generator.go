// Package k8sconfig generates kubeconfig files for Kubernetes clusters using
// CSP CLI tools (aws, gcloud, az). It replaces the previous k8sauth package
// which used Go SDK calls directly.
package k8s

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/IBM/simrun/simrun/internal/envutil"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// CloudType identifies the Kubernetes provider.
type CloudType string

const (
	CloudTypeEKS CloudType = "eks"
	CloudTypeGKE CloudType = "gke"
	CloudTypeAKS CloudType = "aks"
)

// GenerateConfig holds the parameters for kubeconfig generation.
type GenerateConfig struct {
	ConnectorName string            // Used for deterministic file path
	CloudType     CloudType         // Auto-detected from cloud connector
	ClusterName   string
	Region        string
	Project       string            // GKE only
	ResourceGroup string            // AKS only
	Credentials   map[string]string // Cloud credentials as env vars
}

// DeriveCloudType determines the Kubernetes provider from the cloud connector type.
func DeriveCloudType(cloudConnectorType string) (CloudType, error) {
	switch cloudConnectorType {
	case "aws":
		return CloudTypeEKS, nil
	case "gcp":
		return CloudTypeGKE, nil
	case "azure":
		return CloudTypeAKS, nil
	default:
		return "", fmt.Errorf("unsupported cloud connector type for kubernetes: %s (must be aws, gcp, or azure)", cloudConnectorType)
	}
}

// Generate creates a kubeconfig file using CSP CLI tools and returns its path.
// The file path is deterministic: /tmp/simrun-kubeconfig-{connector-name}.yaml
func Generate(ctx context.Context, cfg GenerateConfig) (string, error) {
	path := KubeconfigPath(cfg.ConnectorName)

	switch cfg.CloudType {
	case CloudTypeEKS:
		return generateEKS(ctx, cfg, path)
	case CloudTypeGKE:
		return generateGKE(ctx, cfg, path)
	case CloudTypeAKS:
		return generateAKS(ctx, cfg, path)
	default:
		return "", fmt.Errorf("unsupported cloud type: %s", cfg.CloudType)
	}
}

// ValidateKubeconfig verifies the generated kubeconfig file has the expected structure.
// The actual cluster connectivity is validated by the CLI tools during Generate
// (they call DescribeCluster/list-clusters and fail if credentials or cluster are invalid).
func ValidateKubeconfig(kubeconfigPath string) error {
	kc, err := parseKubeconfig(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	if len(kc.Clusters) == 0 {
		return fmt.Errorf("kubeconfig has no clusters defined")
	}
	if kc.Clusters[0].Cluster.Server == "" {
		return fmt.Errorf("kubeconfig cluster has no server endpoint")
	}
	if len(kc.Users) == 0 {
		return fmt.Errorf("kubeconfig has no users defined")
	}

	return nil
}

// KubeconfigPath returns the deterministic path for a connector's kubeconfig.
func KubeconfigPath(connectorName string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("simrun-kubeconfig-%s.yaml", connectorName))
}

func generateEKS(ctx context.Context, cfg GenerateConfig, outputPath string) (string, error) {
	args := []string{
		"eks", "update-kubeconfig",
		"--name", cfg.ClusterName,
		"--region", cfg.Region,
		"--kubeconfig", outputPath,
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Env = envutil.MergeWithProcessEnv(cfg.Credentials)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("aws eks update-kubeconfig failed: %w\nOutput: %s", err, output)
	}

	log.WithFields(log.Fields{
		"cluster": cfg.ClusterName,
		"region":  cfg.Region,
		"path":    outputPath,
	}).Debug("Generated EKS kubeconfig")

	return outputPath, nil
}

func generateGKE(ctx context.Context, cfg GenerateConfig, outputPath string) (string, error) {
	if cfg.Project == "" {
		return "", fmt.Errorf("project is required for GKE clusters")
	}

	args := []string{
		"container", "clusters", "get-credentials",
		cfg.ClusterName,
		"--region", cfg.Region,
		"--project", cfg.Project,
	}

	cmd := exec.CommandContext(ctx, "gcloud", args...)
	// gcloud uses KUBECONFIG env var to determine output path
	cmd.Env = append(
		envutil.MergeWithProcessEnv(cfg.Credentials),
		"KUBECONFIG="+outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gcloud get-credentials failed: %w\nOutput: %s", err, output)
	}

	log.WithFields(log.Fields{
		"cluster": cfg.ClusterName,
		"region":  cfg.Region,
		"project": cfg.Project,
		"path":    outputPath,
	}).Debug("Generated GKE kubeconfig")

	return outputPath, nil
}

func generateAKS(ctx context.Context, cfg GenerateConfig, outputPath string) (string, error) {
	if cfg.ResourceGroup == "" {
		return "", fmt.Errorf("resource_group is required for AKS clusters")
	}

	args := []string{
		"aks", "get-credentials",
		"--name", cfg.ClusterName,
		"--resource-group", cfg.ResourceGroup,
		"--file", outputPath,
		"--overwrite-existing",
	}

	cmd := exec.CommandContext(ctx, "az", args...)
	cmd.Env = envutil.MergeWithProcessEnv(cfg.Credentials)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("az aks get-credentials failed: %w\nOutput: %s", err, output)
	}

	log.WithFields(log.Fields{
		"cluster":        cfg.ClusterName,
		"resource_group": cfg.ResourceGroup,
		"path":           outputPath,
	}).Debug("Generated AKS kubeconfig")

	return outputPath, nil
}

// kubeconfig types for YAML parsing (used by TestConnection)

type kubeconfigFile struct {
	APIVersion     string              `json:"apiVersion"`
	Kind           string              `json:"kind"`
	Clusters       []kubeconfigCluster `json:"clusters"`
	Users          []kubeconfigUser    `json:"users"`
	Contexts       []kubeconfigContext `json:"contexts"`
	CurrentContext string              `json:"current-context"`
}

type kubeconfigCluster struct {
	Name    string                `json:"name"`
	Cluster kubeconfigClusterData `json:"cluster"`
}

type kubeconfigClusterData struct {
	Server                   string `json:"server"`
	CertificateAuthorityData string `json:"certificate-authority-data"`
}

type kubeconfigUser struct {
	Name string             `json:"name"`
	User kubeconfigUserData `json:"user"`
}

type kubeconfigUserData struct {
	Token string `json:"token,omitempty"`
	Exec  *any   `json:"exec,omitempty"`
}

type kubeconfigContext struct {
	Name    string                `json:"name"`
	Context kubeconfigContextData `json:"context"`
}

type kubeconfigContextData struct {
	Cluster string `json:"cluster"`
	User    string `json:"user"`
}

func parseKubeconfig(path string) (*kubeconfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var kc kubeconfigFile
	if err := yaml.Unmarshal(data, &kc); err != nil {
		return nil, err
	}
	return &kc, nil
}
