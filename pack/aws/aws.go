// Package aws provides AWS SDK helpers for simulation packs.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/simrun/internal/version"
	"github.com/IBM/simrun/pack"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	"github.com/cenkalti/backoff/v5"
)

// AWSConfig creates an AWS SDK v2 config using the default credential chain.
// The AWS SDK automatically reads from standard environment variables:
//   - AWS_ACCESS_KEY_ID: AWS access key ID
//   - AWS_SECRET_ACCESS_KEY: AWS secret access key
//   - AWS_SESSION_TOKEN: AWS session token (optional)
//   - AWS_REGION or AWS_DEFAULT_REGION: AWS region
//   - AWS_PROFILE: AWS profile name
//
// It also supports shared credentials file (~/.aws/credentials) and
// EC2/ECS instance metadata.
func AWSConfig(ctx context.Context) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if executionID := pack.ExecutionIDFromContext(ctx); executionID != "" {
		opts = append(opts, config.WithAPIOptions([]func(*smithymiddleware.Stack) error{
			awsmiddleware.AddUserAgentKeyValue("simrun", version.Version),
			awsmiddleware.AddUserAgentKeyValue("simrun-exec", executionID),
		}))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config: %w", err)
	}
	return cfg, nil
}

// AssumeRoleOption configures role assumption behavior.
type AssumeRoleOption func(*assumeRoleConfig)

type assumeRoleConfig struct {
	initialInterval time.Duration
	maxInterval     time.Duration
	maxElapsedTime  time.Duration
	externalID      string
}

func defaultAssumeRoleConfig() *assumeRoleConfig {
	return &assumeRoleConfig{
		initialInterval: 1 * time.Second,
		maxInterval:     10 * time.Second,
		maxElapsedTime:  1 * time.Minute,
	}
}

// WithRetryBackoff configures retry behavior for role assumption.
// Default: 1s initial, 10s max interval, 1min total time.
func WithRetryBackoff(initial, maxInterval, maxElapsed time.Duration) AssumeRoleOption {
	return func(c *assumeRoleConfig) {
		c.initialInterval = initial
		c.maxInterval = maxInterval
		c.maxElapsedTime = maxElapsed
	}
}

// WithExternalID sets the external ID for role assumption.
func WithExternalID(externalID string) AssumeRoleOption {
	return func(c *assumeRoleConfig) {
		c.externalID = externalID
	}
}

// AssumeAWSRole assumes an AWS IAM role with retry logic for eventual consistency.
// Returns a new config with assumed role credentials.
// The retry logic handles the delay between role creation and assumability.
func AssumeAWSRole(ctx context.Context, cfg aws.Config, roleArn string, opts ...AssumeRoleOption) (aws.Config, error) {
	c := defaultAssumeRoleConfig()
	for _, opt := range opts {
		opt(c)
	}

	stsClient := sts.NewFromConfig(cfg)

	var assumeOpts []func(*stscreds.AssumeRoleOptions)
	if c.externalID != "" {
		assumeOpts = append(assumeOpts, func(o *stscreds.AssumeRoleOptions) {
			o.ExternalID = aws.String(c.externalID)
		})
	}

	assumeRoleProvider := stscreds.NewAssumeRoleProvider(stsClient, roleArn, assumeOpts...)

	backoffStrategy := backoff.NewExponentialBackOff()
	backoffStrategy.InitialInterval = c.initialInterval
	backoffStrategy.Multiplier = 2
	backoffStrategy.MaxInterval = c.maxInterval

	_, err := backoff.Retry(ctx, func() (struct{}, error) {
		_, err := assumeRoleProvider.Retrieve(ctx)
		return struct{}{}, err
	}, backoff.WithBackOff(backoffStrategy), backoff.WithMaxElapsedTime(c.maxElapsedTime))

	if err != nil {
		return aws.Config{}, fmt.Errorf("assume role %s: %w", roleArn, err)
	}

	// Create a new config with the assumed role credentials
	newCfg := cfg.Copy()
	newCfg.Credentials = aws.NewCredentialsCache(assumeRoleProvider)

	return newCfg, nil
}

// RunSSMCommand executes a shell command on an EC2 instance via SSM and waits for completion.
// It combines SendCommand and WaitForOutput into a single call.
// Returns the command output on success.
//
// Example:
//
//	output, err := aws.RunSSMCommand(ctx, ssmClient, instanceID, "echo hello", 30*time.Second)
//	if err != nil {
//	    log.WithError(err).Warn("Command failed")
//	}
func RunSSMCommand(ctx context.Context, ssmClient *ssm.Client, instanceID, command string, timeout time.Duration) (string, error) {
	result, err := ssmClient.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  []string{instanceID},
		Parameters:   map[string][]string{"commands": {command}},
	})
	if err != nil {
		return "", fmt.Errorf("send SSM command: %w", err)
	}

	if result.Command == nil || result.Command.CommandId == nil {
		return "", fmt.Errorf("SSM command response missing command ID")
	}

	commandID := *result.Command.CommandId

	invocationOutput, err := ssm.NewCommandExecutedWaiter(ssmClient).WaitForOutput(ctx,
		&ssm.GetCommandInvocationInput{
			CommandId:  &commandID,
			InstanceId: &instanceID,
		}, timeout)
	if err != nil {
		return "", fmt.Errorf("wait for SSM command: %w", err)
	}

	if invocationOutput != nil && invocationOutput.StandardOutputContent != nil {
		return *invocationOutput.StandardOutputContent, nil
	}

	return "", nil
}
