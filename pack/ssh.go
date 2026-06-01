package pack

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// SSHLoggingEnabledEnvVar enables SSH command output logging when set to "true".
	SSHLoggingEnabledEnvVar = "SR_SSH_LOGGING_ENABLED"
	// SSHLogDirEnvVar specifies the directory for SSH log files.
	SSHLogDirEnvVar = "SR_SSH_LOG_DIR"
)

// SSHConfig configures SSH connection parameters.
type SSHConfig struct {
	Host           string        // Required: IP or hostname
	Username       string        // Required: SSH username
	PrivateKeyPath string        // Path to private key file
	ConnectTimeout time.Duration // Default: 30s
}

// SSHClient represents an SSH connection for executing remote commands.
type SSHClient struct {
	config      SSHConfig
	logger      *logrus.Entry
	logWriter   *os.File // SSH output log file, lazy-initialized
	logInitDone bool     // whether we've attempted to init the log writer
}

// NewSSHClient creates an SSH client from config.
func NewSSHClient(cfg SSHConfig, logger *logrus.Entry) (*SSHClient, error) {
	if cfg.Host == "" {
		return nil, errors.New("SSH host is required")
	}
	if cfg.Username == "" {
		return nil, errors.New("SSH username is required")
	}
	if cfg.PrivateKeyPath == "" {
		return nil, errors.New("SSH private key path is required")
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 30 * time.Second
	}
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}

	return &SSHClient{
		config: cfg,
		logger: logger,
	}, nil
}

// sshArgs returns the common SSH arguments for this client.
func (c *SSHClient) sshArgs() []string {
	return []string{
		"-i", c.config.PrivateKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", fmt.Sprintf("ConnectTimeout=%d", int(c.config.ConnectTimeout.Seconds())),
		fmt.Sprintf("%s@%s", c.config.Username, c.config.Host),
	}
}

// Run executes a command on the remote host and returns output, exit code, and error.
// Exit code is -1 if there was a connection error before the command could run.
func (c *SSHClient) Run(ctx context.Context, command string) (output string, exitCode int, err error) {
	args := append(c.sshArgs(), command)
	return c.exec(ctx, args, nil)
}

// RunScript executes a multi-line script on the remote host.
// The script is passed via stdin to avoid escaping issues.
func (c *SSHClient) RunScript(ctx context.Context, script string) (output string, exitCode int, err error) {
	args := append(c.sshArgs(), "bash -s")
	return c.exec(ctx, args, strings.NewReader(script))
}

// exec runs an SSH command with optional stdin.
func (c *SSHClient) exec(ctx context.Context, args []string, stdin *strings.Reader) (string, int, error) {
	c.initLogWriter()

	sshCmd := exec.CommandContext(ctx, "ssh", args...)
	if stdin != nil {
		sshCmd.Stdin = stdin
	}
	outputBytes, cmdErr := sshCmd.CombinedOutput()
	output := string(outputBytes)

	// Append raw output to log file if logging is enabled
	if c.logWriter != nil && len(outputBytes) > 0 {
		_, _ = c.logWriter.Write(outputBytes)
	}

	if cmdErr != nil {
		var exitErr *exec.ExitError
		if errors.As(cmdErr, &exitErr) {
			return output, exitErr.ExitCode(), cmdErr
		}
		return output, -1, fmt.Errorf("SSH execution failed: %w", cmdErr)
	}

	return output, 0, nil
}

// initLogWriter initializes the SSH log writer on first call.
// If SR_SSH_LOGGING_ENABLED != "true" or SR_SSH_LOG_DIR is unset, this is a no-op.
func (c *SSHClient) initLogWriter() {
	if c.logInitDone {
		return
	}
	c.logInitDone = true

	if os.Getenv(SSHLoggingEnabledEnvVar) != "true" {
		return
	}

	logDir := os.Getenv(SSHLogDirEnvVar)
	if logDir == "" {
		return
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		c.logger.WithError(err).Warn("Failed to create SSH log directory")
		return
	}

	// Use execution_id from logger context for the filename
	executionID := "unknown"
	if id, ok := c.logger.Data["execution_id"].(string); ok && id != "" {
		executionID = id
	}

	logPath := filepath.Join(logDir, executionID+".log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		c.logger.WithError(err).Warn("Failed to create SSH log file")
		return
	}

	c.logWriter = f
	c.logger.WithField("ssh_log_path", logPath).Debug("SSH output logging enabled")
}

// SSHFromTerraform extracts SSH config from terraform outputs.
// It looks for the following standard output names:
//   - attacker_vm_public_ip: Host IP address
//   - attacker_vm_user: SSH username
//   - attacker_vm_private_key_path: Path to private key file
func SSHFromTerraform(outputs map[string]string) (SSHConfig, error) {
	host := outputs["attacker_vm_public_ip"]
	if host == "" {
		return SSHConfig{}, errors.New("missing terraform output: attacker_vm_public_ip")
	}

	user := outputs["attacker_vm_user"]
	if user == "" {
		return SSHConfig{}, errors.New("missing terraform output: attacker_vm_user")
	}

	keyPath := outputs["attacker_vm_private_key_path"]
	if keyPath == "" {
		return SSHConfig{}, errors.New("missing terraform output: attacker_vm_private_key_path")
	}

	return SSHConfig{
		Host:           host,
		Username:       user,
		PrivateKeyPath: keyPath,
	}, nil
}

// SSHClientFromTerraform creates an SSH client from terraform outputs in one call.
// This combines SSHFromTerraform and NewSSHClient into a single convenience function.
func SSHClientFromTerraform(outputs map[string]string, logger *logrus.Entry) (*SSHClient, error) {
	cfg, err := SSHFromTerraform(outputs)
	if err != nil {
		return nil, err
	}
	return NewSSHClient(cfg, logger)
}
