// Package awsauth provides AWS cross-account role assumption for simrun.
// AWS connectors specify a role_arn in their config; simrun assumes the role
// via STS before each scenario and injects temporary credentials into the
// per-run environment.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/cenkalti/backoff/v5"
	log "github.com/sirupsen/logrus"
)

// AssumeRole performs STS AssumeRole with retry logic for eventual consistency.
func AssumeRole(ctx context.Context, roleArn, externalID string) (map[string]string, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithDefaultRegion("us-east-1"),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(awsCfg)

	var assumeOpts []func(*stscreds.AssumeRoleOptions)
	assumeOpts = append(assumeOpts, func(o *stscreds.AssumeRoleOptions) {
		o.Duration = 1 * time.Hour
	})
	if externalID != "" {
		assumeOpts = append(assumeOpts, func(o *stscreds.AssumeRoleOptions) {
			o.ExternalID = aws.String(externalID)
		})
	}

	provider := stscreds.NewAssumeRoleProvider(stsClient, roleArn, assumeOpts...)

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 1 * time.Second
	bo.Multiplier = 2
	bo.MaxInterval = 10 * time.Second

	creds, err := backoff.Retry(ctx, func() (aws.Credentials, error) {
		return provider.Retrieve(ctx)
	}, backoff.WithBackOff(bo), backoff.WithMaxElapsedTime(1*time.Minute))

	if err != nil {
		return nil, fmt.Errorf("failed to assume role %s: %w", roleArn, err)
	}

	log.WithFields(log.Fields{
		"role_arn": roleArn,
		"expires":  creds.Expires.Format(time.RFC3339),
	}).Debug("Successfully assumed AWS role")

	return map[string]string{
		"AWS_ACCESS_KEY_ID":     creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
		"AWS_SESSION_TOKEN":     creds.SessionToken,
	}, nil
}

// ResolveCredentials resolves the current AWS credentials using the default
// credential chain (which supports IRSA, instance profiles, env vars, etc.)
// and returns them as a map suitable for environment variable injection.
// This is needed for GCP WIF on EKS: the Google Cloud external account library
// only reads AWS credentials from env vars or IMDS, but IRSA provides
// credentials via AWS_WEB_IDENTITY_TOKEN_FILE which the Google library doesn't
// understand.
func ResolveCredentials(ctx context.Context) (map[string]string, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	creds, err := awsCfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	result := map[string]string{
		"AWS_ACCESS_KEY_ID":     creds.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
	}
	if creds.SessionToken != "" {
		result["AWS_SESSION_TOKEN"] = creds.SessionToken
	}

	return result, nil
}
