// Package executor handles pack protocol communication using PackRunners.
package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/IBM/simrun/internal/packs/runner"
	"github.com/IBM/simrun/pack"
	"github.com/sirupsen/logrus"
)

// Executor handles pack protocol communication using a PackRunner.
type Executor struct {
	runner          runner.PackRunner
	packLogsEnabled bool
	baseFields      logrus.Fields
}

// NewExecutor creates a new Executor with the given runner.
// packLogsEnabled controls whether stderr output from the pack is logged.
func NewExecutor(r runner.PackRunner, packLogsEnabled bool) *Executor {
	return &Executor{runner: r, packLogsEnabled: packLogsEnabled}
}

// WithLogFields returns a new Executor that includes extra fields in all log entries.
func (e *Executor) WithLogFields(fields logrus.Fields) *Executor {
	return &Executor{runner: e.runner, packLogsEnabled: e.packLogsEnabled, baseFields: fields}
}

// Detonate runs the pack's detonate command.
func (e *Executor) Detonate(ctx context.Context, input *pack.DetonateInput) (*pack.Result, error) {
	var result pack.Result
	if err := e.executeCommand(ctx, "detonate", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Cleanup runs the pack's cleanup command.
func (e *Executor) Cleanup(ctx context.Context, input *pack.CleanupInput) (*pack.Result, error) {
	var result pack.Result
	if err := e.executeCommand(ctx, "cleanup", input, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// executeCommand is a generic helper to marshal input, run command, and unmarshal output.
func (e *Executor) executeCommand(ctx context.Context, command string, input any, output any) error {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal %s input: %w", command, err)
	}

	outputJSON, err := e.runCommandWithStderr(ctx, command, inputJSON)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(outputJSON, output); err != nil {
		return fmt.Errorf("failed to parse %s output: %w", command, err)
	}

	return nil
}

// runCommandWithStderr executes a command and processes stderr output.
func (e *Executor) runCommandWithStderr(ctx context.Context, command string, input []byte) ([]byte, error) {
	output, err := e.runner.RunCommand(ctx, command, input)

	// Process stderr after command completes
	stderr := e.runner.GetStderr()
	if stderr != nil {
		e.processStderr(stderr)
	}

	return output, err
}

// processStderr reads stderr line by line and logs appropriately.
func (e *Executor) processStderr(stderr io.Reader) {
	if !e.packLogsEnabled {
		return
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Try to parse as JSON log line
		var logLine pack.LogLine
		if err := json.Unmarshal([]byte(line), &logLine); err == nil {
			// Valid JSON log line
			e.logPackMessage(logLine)
		} else {
			// Plain text line
			entry := logrus.WithFields(e.baseFields)
			entry.WithField("log_source", "pack").WithField("runner", e.runner.String()).Info(line)
		}
	}
}

// logPackMessage logs a structured log line from the pack.
func (e *Executor) logPackMessage(logLine pack.LogLine) {
	fields := e.buildLogFields(logLine)
	entry := logrus.WithFields(fields)

	switch logLine.Level {
	case pack.LogLevelDebug:
		entry.Debug(logLine.Msg)
	case pack.LogLevelInfo:
		entry.Info(logLine.Msg)
	case pack.LogLevelWarn:
		entry.Warn(logLine.Msg)
	case pack.LogLevelError:
		entry.Error(logLine.Msg)
	default:
		entry.Info(logLine.Msg)
	}
}

// buildLogFields constructs log fields from a pack log line
func (e *Executor) buildLogFields(logLine pack.LogLine) logrus.Fields {
	fields := logrus.Fields{
		"log_source": "pack",
	}

	// Include base fields (e.g. run_id)
	for k, v := range e.baseFields {
		fields[k] = v
	}

	// Add standard fields if present
	if logLine.Pack != "" {
		fields["pack"] = logLine.Pack
	}
	if logLine.PackVersion != "" {
		fields["pack_version"] = logLine.PackVersion
	}
	if logLine.Simulation != "" {
		fields["simulation"] = logLine.Simulation
	}
	if logLine.ExecutionID != "" {
		fields["execution_id"] = logLine.ExecutionID
	}

	// Add extra fields from the log line
	for k, v := range logLine.Extra {
		fields[k] = v
	}

	return fields
}
