// Package azure provides Azure SDK helpers for simulation packs.
package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/IBM/simrun/pack"
)

// AzureCredential creates an Azure token credential using the default credential chain.
// The Azure SDK automatically reads from standard environment variables:
//   - AZURE_TENANT_ID: Azure tenant ID
//   - AZURE_CLIENT_ID: Azure client (application) ID
//   - AZURE_CLIENT_SECRET: Azure client secret
//
// It also supports managed identity, Azure CLI credentials, and other authentication methods.
func AzureCredential(ctx context.Context) (azcore.TokenCredential, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create Azure credential: %w", err)
	}
	return cred, nil
}

// ClientOptions returns Azure client options with a custom User-Agent header
// if an execution ID is present in the context.
// Azure SDK limits ApplicationID to 24 characters.
func ClientOptions(ctx context.Context) *arm.ClientOptions {
	opts := &arm.ClientOptions{}
	if executionID := pack.ExecutionIDFromContext(ctx); executionID != "" {
		opts.Telemetry.ApplicationID = "sr-" + executionID
	}
	return opts
}
