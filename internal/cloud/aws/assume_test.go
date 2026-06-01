package aws

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAssumeRole_CredentialsShape is gated behind SR_AWS_TEST_ROLE_ARN. It
// documents the contract that AssumeRole returns the three env-var keys that
// pack runners inject. CI skips it; engineers run it locally with a real role.
func TestAssumeRole_CredentialsShape(t *testing.T) {
	arn := os.Getenv("SR_AWS_TEST_ROLE_ARN")
	if arn == "" {
		t.Skip("SR_AWS_TEST_ROLE_ARN not set; skipping live STS test")
	}
	creds, err := AssumeRole(context.Background(), arn, os.Getenv("SR_AWS_TEST_EXTERNAL_ID"))
	require.NoError(t, err)
	assert.NotEmpty(t, creds["AWS_ACCESS_KEY_ID"])
	assert.NotEmpty(t, creds["AWS_SECRET_ACCESS_KEY"])
	assert.NotEmpty(t, creds["AWS_SESSION_TOKEN"])
}
