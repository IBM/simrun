// Package runner provides abstractions for executing pack commands.
package runner

import (
	"context"
	"fmt"
	"io"

	"github.com/IBM/simrun/simrun/internal/config"
	"github.com/IBM/simrun/simrun/pack"
)

// PackRunner abstracts the execution of pack commands.
type PackRunner interface {
	// RunCommand executes a pack command with JSON input on stdin
	// and returns JSON output from stdout. Stderr is captured internally
	// and can be retrieved via GetStderr().
	RunCommand(ctx context.Context, command string, input []byte) ([]byte, error)

	// GetStderr returns the captured stderr output after RunCommand completes.
	GetStderr() io.Reader

	// String returns a description of the runner for logging.
	String() string

	// Close cleans up any resources held by the runner.
	Close() error
}

// PackRunnerFactory creates PackRunners based on pack configuration.
type PackRunnerFactory interface {
	// CreateRunner returns the appropriate runner for the pack type.
	CreateRunner(ctx context.Context, cfg config.PackConfig) (PackRunner, error)

	// GetManifest retrieves the manifest for a pack.
	// Parameters are optional key-value configuration passed to the pack via stdin.
	GetManifest(ctx context.Context, cfg config.PackConfig, parameters map[string]any) (*pack.ManifestResponse, error)
}

// handleExitCode processes pack exit codes and returns output or error.
// Exit code 1 with output is considered a simulation error with valid JSON response.
// Exit code 2 is a protocol error.
func handleExitCode(exitCode int, output []byte) ([]byte, error) {
	switch exitCode {
	case 1:
		// Simulation error - output should still be valid JSON
		if len(output) > 0 {
			return output, nil
		}
		return nil, fmt.Errorf("simulation error (exit code %d)", exitCode)
	case 2:
		return nil, fmt.Errorf("protocol error (exit code %d): invalid input or protocol violation", exitCode)
	default:
		return nil, fmt.Errorf("command failed with exit code %d", exitCode)
	}
}
