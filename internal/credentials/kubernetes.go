package credentials

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/simrun/internal/cloud/k8s"
	"github.com/IBM/simrun/internal/db"
	log "github.com/sirupsen/logrus"
)

// buildKubernetesCredentials generates a kubeconfig for a Kubernetes connector
// by resolving its referenced cloud connector credentials and invoking CSP CLI
// tools, then returns the KUBECONFIG/KUBE_CONFIG_PATH env-var pair.
func (r *Resolver) buildKubernetesCredentials(ctx context.Context, connector *db.Connector) (map[string]string, error) {
	var cfg map[string]any
	if err := json.Unmarshal(connector.Config, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse kubernetes config: %w", err)
	}

	clusterName := strVal(cfg, "cluster_name")
	region := strVal(cfg, "region")
	cloudConnectorName := strVal(cfg, "cloud_connector")
	resourceGroup := strVal(cfg, "resource_group")
	project := strVal(cfg, "project")

	if clusterName == "" || region == "" || cloudConnectorName == "" {
		return nil, fmt.Errorf("cluster_name, region, and cloud_connector are required")
	}

	// Look up the referenced cloud connector
	cloudConnector, err := r.connectorStore.GetByName(ctx, cloudConnectorName)
	if err != nil {
		return nil, fmt.Errorf("cloud connector '%s' not found: %w", cloudConnectorName, err)
	}
	if !cloudConnector.Enabled {
		return nil, fmt.Errorf("cloud connector '%s' is disabled", cloudConnectorName)
	}

	// Derive cloud type from the cloud connector
	cloudType, err := k8s.DeriveCloudType(cloudConnector.Type)
	if err != nil {
		return nil, err
	}

	// Build cloud credentials from the referenced connector
	cloudCreds, err := r.Build(ctx, cloudConnector)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cloud connector credentials: %w", err)
	}

	// For GKE, derive project from GCP connector config if not explicitly set
	if cloudType == k8s.CloudTypeGKE && project == "" {
		var gcpCfg map[string]any
		if err := json.Unmarshal(cloudConnector.Config, &gcpCfg); err == nil {
			project = strVal(gcpCfg, "project_id")
		}
	}

	kubeconfigPath, err := k8s.Generate(ctx, k8s.GenerateConfig{
		ConnectorName: cloudConnectorName,
		CloudType:     cloudType,
		ClusterName:   clusterName,
		Region:        region,
		Project:       project,
		ResourceGroup: resourceGroup,
		Credentials:   cloudCreds,
	})
	if err != nil {
		return nil, err
	}

	log.WithField("kubeconfig", kubeconfigPath).Debug("Resolved Kubernetes credentials")

	return map[string]string{
		"KUBECONFIG":       kubeconfigPath,
		"KUBE_CONFIG_PATH": kubeconfigPath,
	}, nil
}
