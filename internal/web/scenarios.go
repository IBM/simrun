package web

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/IBM/simrun/internal/config"
	"github.com/IBM/simrun/internal/credentials"
	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/parser"
	"github.com/IBM/simrun/internal/results"
	"github.com/IBM/simrun/internal/runner"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// ScenarioService handles scenario parsing, execution, and result persistence.
type ScenarioService struct {
	runStore       db.RunStore
	scenarioStore  db.ScenarioStore
	packStore      db.PackStore
	configStore    db.ConfigStore
	creds          *credentials.Resolver
	exporter       *ResultExporter
	hub            *Hub
	runLogRegistry *RunLogRegistry
	dataDir        string
}

// NewScenarioService creates a new ScenarioService.
func NewScenarioService(runStore db.RunStore, scenarioStore db.ScenarioStore, packStore db.PackStore, configStore db.ConfigStore, creds *credentials.Resolver, exporter *ResultExporter, hub *Hub, runLogRegistry *RunLogRegistry, dataDir string) *ScenarioService {
	return &ScenarioService{
		runStore:       runStore,
		scenarioStore:  scenarioStore,
		packStore:      packStore,
		configStore:    configStore,
		creds:          creds,
		exporter:       exporter,
		hub:            hub,
		runLogRegistry: runLogRegistry,
		dataDir:        dataDir,
	}
}

// loadPacksFromDB returns the pack list as []config.PackConfig sourced entirely
// from the database. Replaces populatePackParameters which merged YAML+DB.
func (s *ScenarioService) loadPacksFromDB(ctx context.Context) ([]config.PackConfig, error) {
	if s.packStore == nil {
		return nil, nil
	}
	dbPacks, err := s.packStore.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]config.PackConfig, 0, len(dbPacks))
	for _, p := range dbPacks {
		out = append(out, config.PackConfig{
			Name:       p.Name,
			Type:       config.PackType(p.Type),
			Source:     p.Source,
			Version:    p.Version,
			Parameters: p.Parameters,
		})
	}
	return out, nil
}

// Lint parses YAML and returns a summary without executing.
func (s *ScenarioService) Lint(yamlContent []byte) (*LintResponse, error) {
	ctx := context.Background()
	packs, err := s.loadPacksFromDB(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load packs: %w", err)
	}
	parseOpts := &parser.ParseOptions{
		Packs:   packs,
		DataDir: s.dataDir,
	}

	parseResult, err := parser.ParseWithOptions(yamlContent, parseOpts)
	if err != nil {
		return &LintResponse{
			Valid: false,
			Error: err.Error(),
		}, nil
	}

	var linted []LintedScenario
	for _, sc := range parseResult.Scenarios {
		executorType := "unknown"
		executorName := "unknown"
		if sc.Detonator != nil {
			executorType = "detonator"
			executorName = sc.Detonator.String()
		} else if sc.Injector != nil {
			executorType = "injector"
			executorName = sc.Injector.String()
		}

		linted = append(linted, LintedScenario{
			Name:         sc.Name,
			ExecutorType: executorType,
			ExecutorName: executorName,
			Assertions:   len(sc.Assertions),
		})
	}

	return &LintResponse{
		Valid:     true,
		Scenarios: linted,
	}, nil
}

// RunOptions contains optional parameters for running scenarios.
type RunOptions struct {
	Parallelism   int
	ScheduleID    *uuid.UUID
	ScheduleName  *string
	CreatedBy     string
	ExploreMode   bool
	CleanupAlerts bool
	Timeout       time.Duration // global timeout override; 0 means use per-scenario YAML timeout
}

// Run starts async scenario execution. Returns the runId immediately.
// It fetches the scenario YAML from the database using the provided scenarioID.
func (s *ScenarioService) Run(ctx context.Context, scenarioID uuid.UUID, opts *RunOptions) (string, error) {
	savedScenario, err := s.scenarioStore.Get(ctx, scenarioID)
	if err != nil {
		return "", fmt.Errorf("scenario not found: %w", err)
	}

	appCfg := config.DefaultAppConfig()
	if s.configStore != nil {
		loaded, err := s.configStore.GetAppConfig(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to load app config: %w", err)
		}
		appCfg = loaded
	}
	packs, err := s.loadPacksFromDB(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load packs: %w", err)
	}

	if opts == nil {
		opts = &RunOptions{}
	}
	parallelism := opts.Parallelism
	if parallelism <= 0 {
		parallelism = appCfg.Parallelism
	}

	// Build run-specific env vars from secrets and connector config.
	// These are threaded through the execution chain instead of mutating
	// global process env, so concurrent runs don't interfere with each other.
	runEnv := make(map[string]string)
	if appCfg.SSHLoggingEnabled {
		runEnv["SR_SSH_LOG_DIR"] = filepath.Join(s.dataDir, "ssh-logs")
	}
	for key, val := range s.creds.ResolveElasticEnv(ctx) {
		runEnv[key] = val
	}
	for key, val := range s.creds.LoadAllSecrets(ctx) {
		runEnv[key] = val
	}

	parseOpts := &parser.ParseOptions{
		Packs:            packs,
		EnvVars:          runEnv,
		DataDir:          s.dataDir,
		TerraformVersion: appCfg.TerraformVersion,
		PackLogsEnabled:  appCfg.PackLogsEnabled,
	}

	parseResult, err := parser.ParseWithOptions([]byte(savedScenario.YAML), parseOpts)
	if err != nil {
		return "", err
	}
	scenarios := parseResult.Scenarios

	// Resolve target credentials from top-level targets map and merge into runEnv
	targetCreds, err := s.creds.BuildTargets(ctx, parseResult.Targets)
	if err != nil {
		return "", err
	}
	for key, val := range targetCreds {
		runEnv[key] = val
	}

	runID := uuid.New()
	now := time.Now()

	run := &db.Run{
		ID:           runID,
		Status:       "running",
		StartTime:    now,
		Total:        len(scenarios),
		ScenarioID:   &scenarioID,
		ScheduleID:   opts.ScheduleID,
		ScheduleName: opts.ScheduleName,
		CreatedBy:    opts.CreatedBy,
	}

	if err := s.runStore.Create(ctx, run); err != nil {
		return "", err
	}

	// Insert pending scenario rows and build name→ID map for status updates
	scenarioDBIDs := make(map[string]uuid.UUID, len(scenarios))
	for _, sc := range scenarios {
		dbID, err := s.runStore.CreateScenarioStatus(ctx, runID, sc.Name)
		if err != nil {
			log.WithField("scenario", sc.Name).WithError(err).Error("Failed to create scenario status row")
			continue
		}
		scenarioDBIDs[sc.Name] = dbID
	}

	// Set RunID, EnvVars, ExploreMode, and StatusCallback on all parsed scenarios
	for _, sc := range scenarios {
		sc.RunID = runID.String()
		sc.EnvVars = runEnv
		sc.ExploreMode = opts.ExploreMode
		sc.CleanupAlerts = opts.CleanupAlerts
		if opts.Timeout > 0 {
			sc.Timeout = opts.Timeout
		}
		sc.StatusCallback = func(scenarioName, phase string) {
			if dbID, ok := scenarioDBIDs[scenarioName]; ok {
				if err := s.runStore.UpdateScenarioPhase(context.Background(), dbID, phase); err != nil {
					log.WithField("scenario", scenarioName).WithError(err).Warn("Failed to update scenario phase")
				}
			}
		}
		sc.IdentityCallback = func(scenarioName string, identity runner.ScenarioIdentity) {
			if dbID, ok := scenarioDBIDs[scenarioName]; ok {
				if err := s.runStore.UpdateScenarioIdentity(context.Background(), dbID,
					identity.ExecutorName, identity.ExecutorType, identity.ExecutionID, identity.SimulationID); err != nil {
					log.WithField("scenario", scenarioName).WithError(err).Warn("Failed to update scenario identity")
				}
			}
		}
		sc.AssertionsCallback = func(scenarioName string, assertions []runner.AssertionResult) {
			dbID, ok := scenarioDBIDs[scenarioName]
			if !ok {
				return
			}
			assertionsJSON, err := buildPartialAssertionsJSON(assertions)
			if err != nil {
				log.WithField("scenario", scenarioName).WithError(err).Warn("Failed to marshal partial assertions")
				return
			}
			if err := s.runStore.UpdateScenarioAssertions(context.Background(), dbID, assertionsJSON); err != nil {
				log.WithField("scenario", scenarioName).WithError(err).Warn("Failed to update scenario assertions")
			}
		}
	}

	// Create per-run log writer
	var runLogWriter *RunLogWriter
	if s.runLogRegistry != nil {
		w, err := NewRunLogWriter(s.dataDir, runID.String())
		if err != nil {
			log.WithError(err).Warn("Failed to create run log writer")
		} else {
			runLogWriter = w
			s.runLogRegistry.Register(runID.String(), w)
		}
	}

	// Run scenarios asynchronously
	go func() {
		defer func() {
			if s.runLogRegistry != nil && runLogWriter != nil {
				s.runLogRegistry.Unregister(runID.String())
			}
		}()

		allResults := results.RunScenariosParallel(scenarios, parallelism, func(result *results.ScenarioRunResult) {
			// Update run counters incrementally
			successDelta, failDelta := 0, 0
			if result.Success {
				successDelta = 1
			} else {
				failDelta = 1
			}
			if err := s.runStore.IncrementRunCounters(context.Background(), runID, successDelta, failDelta); err != nil {
				log.WithError(err).Error("Failed to increment run counters")
			}

			row := buildScenarioResultRow(runID, result)

			// Update existing row (created at queue time) with full results
			if dbID, ok := scenarioDBIDs[result.Name]; ok {
				if err := s.runStore.CompleteScenarioResult(context.Background(), dbID, row); err != nil {
					log.WithError(err).Error("Failed to complete scenario result")
				}
			} else {
				// Fallback: insert if no pre-created row exists
				if err := s.runStore.AddScenarioResult(context.Background(), runID, row); err != nil {
					log.WithError(err).Error("Failed to persist scenario result")
				}
			}
		})

		// Mark run as completed (counters were already updated incrementally)
		endTime := time.Now()
		if err := s.runStore.CompleteRun(context.Background(), runID, &endTime); err != nil {
			log.WithError(err).Error("Failed to update run status")
		}

		// Export results to Elasticsearch if any connector has export enabled
		if s.exporter != nil {
			s.exporter.Export(context.Background(), runID, allResults)
		}
	}()

	return runID.String(), nil
}
