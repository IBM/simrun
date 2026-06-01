package detonators

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/IBM/simrun/internal/envutil"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

/*
AWSCLIDetonator allows to execute arbitrary AWS CLI commands, pre-configured to inject the detonation UUID
in the user-agent.
*/
type AWSCLIDetonator struct {
	Script         string
	statusCallback func(phase string)
	envVars        map[string]string
}

func NewAWSCLIDetonator(script string) *AWSCLIDetonator {
	return &AWSCLIDetonator{Script: script}
}

func (m *AWSCLIDetonator) Detonate() (map[string]string, error) {
	if m.statusCallback != nil {
		m.statusCallback("detonating")
	}
	detonationUuid := uuid.New()

	// Sanity check: are we authenticated to AWS?
	awsConfig, err := m.loadAWSConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS configuration: %v", err)
	}
	_, err = awsConfig.Credentials.Retrieve(context.Background())
	if err != nil {
		return nil, fmt.Errorf("you are not authenticated to AWS")
	}

	cmd := exec.Command("bash", "-c", m.Script)
	cmd.Env = append(envutil.MergeWithProcessEnv(m.envVars), "AWS_EXECUTION_ENV=simrun_"+detonationUuid.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("AWS CLI script failed. Output shown below:\n%s", output)
	}

	log.Infof("Execution ID: %s", detonationUuid)

	return map[string]string{"execution_id": detonationUuid.String()}, nil
}

// loadAWSConfig loads the AWS config using explicit env vars if available,
// falling back to default credential resolution.
func (m *AWSCLIDetonator) loadAWSConfig() (aws.Config, error) {
	var opts []func(*config.LoadOptions) error
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
	return config.LoadDefaultConfig(context.Background(), opts...)
}

func (m *AWSCLIDetonator) String() string {
	return "AWSCLIDetonator"
}

func (m *AWSCLIDetonator) SimulationId() string {
	return "AWSCLICommandSimulation"
}

func (m *AWSCLIDetonator) CloudProvider() string {
	return "aws"
}

func (m *AWSCLIDetonator) PackName() string {
	return ""
}

func (m *AWSCLIDetonator) SetRunID(runID string) {}

func (m *AWSCLIDetonator) SetStatusCallback(callback func(phase string)) {
	m.statusCallback = callback
}

func (m *AWSCLIDetonator) SetEnvVars(envVars map[string]string) {
	m.envVars = envVars
}
