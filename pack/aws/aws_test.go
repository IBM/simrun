package aws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IBM/simrun/pack"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func TestAWSConfig_UserAgent(t *testing.T) {
	var capturedUserAgent string

	// Start a fake HTTP server to capture the User-Agent header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserAgent = r.Header.Get("User-Agent")
		// Return a minimal valid SSM SendCommand response
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(200)
		w.Write([]byte(`{"Command":{"CommandId":"test-id"}}`))
	}))
	defer server.Close()

	ctx := context.Background()
	ctx = pack.WithExecutionID(ctx, "test-exec-123")

	cfg, err := AWSConfig(ctx)
	if err != nil {
		t.Fatalf("AWSConfig() error: %v", err)
	}

	// Override endpoint to use our test server
	ssmClient := ssm.NewFromConfig(cfg, func(o *ssm.Options) {
		o.BaseEndpoint = aws.String(server.URL)
		o.Region = "us-east-1"
		o.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "test",
				SecretAccessKey: "test",
				SessionToken:    "test",
			}, nil
		})
	})

	_, _ = ssmClient.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  []string{"i-test"},
		Parameters:   map[string][]string{"commands": {"echo hello"}},
	})

	t.Logf("Captured User-Agent: %s", capturedUserAgent)

	if !strings.Contains(capturedUserAgent, "simrun/") {
		t.Errorf("User-Agent missing 'simrun/' prefix: %s", capturedUserAgent)
	}
	if !strings.Contains(capturedUserAgent, "simrun-exec/test-exec-123") {
		t.Errorf("User-Agent missing 'simrun-exec/test-exec-123': %s", capturedUserAgent)
	}
}

func TestAWSConfig_NoExecutionID(t *testing.T) {
	var capturedUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(200)
		w.Write([]byte(`{"Command":{"CommandId":"test-id"}}`))
	}))
	defer server.Close()

	ctx := context.Background()
	// No execution ID in context

	cfg, err := AWSConfig(ctx)
	if err != nil {
		t.Fatalf("AWSConfig() error: %v", err)
	}

	ssmClient := ssm.NewFromConfig(cfg, func(o *ssm.Options) {
		o.BaseEndpoint = aws.String(server.URL)
		o.Region = "us-east-1"
		o.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "test",
				SecretAccessKey: "test",
				SessionToken:    "test",
			}, nil
		})
	})

	_, _ = ssmClient.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  []string{"i-test"},
		Parameters:   map[string][]string{"commands": {"echo hello"}},
	})

	t.Logf("Captured User-Agent (no exec ID): %s", capturedUserAgent)

	if strings.Contains(capturedUserAgent, "simrun/") {
		t.Errorf("User-Agent should NOT contain 'simrun/' when no execution ID: %s", capturedUserAgent)
	}
}
