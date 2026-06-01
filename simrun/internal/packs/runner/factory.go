// Package runner builds and runs pack binaries (local, uploaded, or remote)
// behind a common interface.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/IBM/simrun/simrun/internal/config"
	"github.com/IBM/simrun/simrun/internal/packs/resolver"
	"github.com/IBM/simrun/simrun/pack"
	"github.com/sirupsen/logrus"
)

// Factory creates PackRunners based on pack configuration.
type Factory struct {
	binaryResolver *resolver.Resolver
}

// NewFactory creates a new runner factory rooted at dataDir. SSH-log
// routing is the caller's responsibility: set SR_SSH_LOG_DIR in the per-run
// envVars map handed to CreateRunner if you want the pack SDK to write SSH
// command logs.
func NewFactory(dataDir string) (*Factory, error) {
	r, err := resolver.NewResolver(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create binary resolver: %w", err)
	}
	return &Factory{binaryResolver: r}, nil
}

// CreateRunner returns the appropriate runner for the pack type.
// envVars are run-specific environment variables that override the process env.
// Pass nil to inherit the process environment as-is.
func (f *Factory) CreateRunner(ctx context.Context, cfg config.PackConfig, envVars map[string]string) (PackRunner, error) {
	switch cfg.Type {
	case config.PackTypeLocal, config.PackTypeUpload:
		if cfg.Source == "" {
			return nil, fmt.Errorf("pack %s: source (path) is required for %s packs", cfg.Name, cfg.Type)
		}
		if _, err := os.Stat(cfg.Source); err != nil {
			return nil, fmt.Errorf("pack binary not found at %s: %w", cfg.Source, err)
		}
		logrus.WithFields(logrus.Fields{
			"pack": cfg.Name,
			"type": string(cfg.Type),
			"path": cfg.Source,
		}).Debug("Using local/uploaded pack binary")
		return NewBinaryRunner(cfg.Source, envVars), nil

	case config.PackTypeRemote:
		resolverCfg := resolver.PackConfig{
			Name:    cfg.Name,
			Source:  cfg.Source,
			Version: cfg.Version,
		}
		packPath, err := f.binaryResolver.Resolve(ctx, resolverCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve remote pack: %w", err)
		}
		return NewBinaryRunner(packPath, envVars), nil

	default:
		return nil, fmt.Errorf("pack %s: unknown type %q", cfg.Name, cfg.Type)
	}
}

// GetManifest retrieves the manifest for a pack by running the manifest command.
// Parameters are optional key-value configuration passed to the pack via stdin.
// envVars are run-specific environment variables (pass nil for CLI/management paths).
func (f *Factory) GetManifest(ctx context.Context, cfg config.PackConfig, parameters map[string]any, envVars map[string]string) (*pack.ManifestResponse, error) {
	runner, err := f.CreateRunner(ctx, cfg, envVars)
	if err != nil {
		return nil, err
	}
	defer runner.Close()

	// Marshal parameters as ManifestInput for the pack's stdin
	var input []byte
	if len(parameters) > 0 {
		manifestInput := pack.ManifestInput{Parameters: parameters}
		input, err = json.Marshal(manifestInput)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal manifest input: %w", err)
		}
	}

	output, err := runner.RunCommand(ctx, "manifest", input)
	if err != nil {
		return nil, fmt.Errorf("manifest command failed: %w", err)
	}

	var manifest pack.ManifestResponse
	if err := json.Unmarshal(output, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}
