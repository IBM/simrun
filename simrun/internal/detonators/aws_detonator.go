package detonators

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/google/uuid"
)

/*
The AWS Detonator allows to send arbitrary requests using the AWS SDK, pre-configured to inject the detonation UUID
in the user-agent.
*/
type AWSDetonator struct {
	DetonationFunc func(awsConfig aws.Config, detonationUuid uuid.UUID) error
	statusCallback func(phase string)
	envVars        map[string]string
}

func NewAWSDetonator(DetonationFunc func(aws.Config, uuid.UUID) error) *AWSDetonator {
	return &AWSDetonator{DetonationFunc: DetonationFunc}
}

func (m *AWSDetonator) Detonate() (map[string]string, error) {
	if m.statusCallback != nil {
		m.statusCallback("detonating")
	}
	detonationUuid := uuid.New()

	opts := []func(*config.LoadOptions) error{customUserAgentApiOptions(detonationUuid)}
	if m.envVars != nil {
		if keyID := m.envVars["AWS_ACCESS_KEY_ID"]; keyID != "" {
			secretKey := m.envVars["AWS_SECRET_ACCESS_KEY"]
			sessionToken := m.envVars["AWS_SESSION_TOKEN"]
			opts = append(opts, config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(keyID, secretKey, sessionToken),
			))
		}
		if region := m.envVars["AWS_REGION"]; region != "" {
			opts = append(opts, config.WithRegion(region))
		} else if region := m.envVars["AWS_DEFAULT_REGION"]; region != "" {
			opts = append(opts, config.WithRegion(region))
		}
	}

	awsConfig, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to authenticate to AWS: %v", err)
	}

	if err := m.DetonationFunc(awsConfig, detonationUuid); err != nil {
		return nil, err
	}

	return map[string]string{"execution_id": detonationUuid.String()}, nil
}

func (m *AWSDetonator) String() string {
	return "AWSDetonator"
}

func (m *AWSDetonator) SimulationId() string {
	return "AWSSDKSimulation"
}

func (m *AWSDetonator) CloudProvider() string {
	return "aws"
}

func (m *AWSDetonator) PackName() string {
	return ""
}

func (m *AWSDetonator) SetRunID(runID string) {}

func (m *AWSDetonator) SetStatusCallback(callback func(phase string)) {
	m.statusCallback = callback
}

func (m *AWSDetonator) SetEnvVars(envVars map[string]string) {
	m.envVars = envVars
}

// Functions below are related to customization of the user-agent header
// Code mostly taken from https://github.com/aws/aws-sdk-go-v2/issues/1432

func customUserAgentApiOptions(uniqueCorrelationId uuid.UUID) config.LoadOptionsFunc {
	return config.WithAPIOptions(func() (v []func(stack *middleware.Stack) error) {
		v = append(v, func(stack *middleware.Stack) error {
			return stack.Build.Add(customUserAgentMiddleware(uniqueCorrelationId), middleware.After)
		})
		return v
	}())
}

func customUserAgentMiddleware(uniqueId uuid.UUID) middleware.BuildMiddleware {
	return middleware.BuildMiddlewareFunc("CustomerUserAgent", func(
		ctx context.Context, input middleware.BuildInput, next middleware.BuildHandler,
	) (out middleware.BuildOutput, metadata middleware.Metadata, err error) {
		request, ok := input.Request.(*smithyhttp.Request)
		if !ok {
			return out, metadata, fmt.Errorf("unknown transport type %T", input.Request)
		}
		request.Header.Set("User-Agent", fmt.Sprintf("%s", "simrun_"+uniqueId.String()))

		return next.HandleBuild(ctx, input)
	})
}
