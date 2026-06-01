package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/IBM/simrun/internal/envutil"
)

// BinaryRunner executes pack commands using a local binary.
type BinaryRunner struct {
	packPath  string
	stderrBuf *bytes.Buffer
	envVars   map[string]string // run-specific env vars (nil = use process env)
}

// NewBinaryRunner creates a runner for a binary pack.
// envVars are run-specific environment variables merged with the process env.
// Pass nil to inherit the process environment as-is (CLI path).
func NewBinaryRunner(packPath string, envVars map[string]string) *BinaryRunner {
	return &BinaryRunner{
		packPath:  packPath,
		stderrBuf: new(bytes.Buffer),
		envVars:   envVars,
	}
}

// RunCommand executes a pack command with input on stdin.
func (r *BinaryRunner) RunCommand(ctx context.Context, command string, input []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.packPath, command)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = envutil.MergeWithProcessEnv(r.envVars)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// Reset stderr buffer for this command
	r.stderrBuf.Reset()
	cmd.Stderr = r.stderrBuf

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return handleExitCode(exitErr.ExitCode(), stdout.Bytes())
		}
		return nil, fmt.Errorf("command failed: %w", err)
	}

	return stdout.Bytes(), nil
}

// GetStderr returns the captured stderr output.
func (r *BinaryRunner) GetStderr() io.Reader {
	return r.stderrBuf
}

// String returns a description of the runner.
func (r *BinaryRunner) String() string {
	return fmt.Sprintf("BinaryRunner(%s)", r.packPath)
}

// Close cleans up any resources. For binary runners, this is a no-op.
func (r *BinaryRunner) Close() error {
	return nil
}
