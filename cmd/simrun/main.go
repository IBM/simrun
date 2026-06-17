// Command simrun is the ASP server: a web server with an embedded SvelteKit
// frontend that runs attack simulations and verifies expected security alerts.
package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/simrun/internal/config"
	"github.com/IBM/simrun/internal/credentials"
	"github.com/IBM/simrun/internal/crypto"
	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/web"
	"github.com/IBM/simrun/internal/web/auth"
	log "github.com/sirupsen/logrus"
)

func main() {
	bootstrap, err := config.LoadBootstrap()
	if err != nil {
		log.Fatalf("Failed to load bootstrap: %v", err)
	}

	if bootstrap.Debug {
		log.SetLevel(log.DebugLevel)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Connect to Postgres
	database, err := db.New(ctx, bootstrap.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Create stores
	runStore := db.NewRunStore(database.Pool)
	scenarioStore := db.NewScenarioStore(database.Pool)
	packStore := db.NewPackStore(database.Pool)
	configStore := db.NewConfigStore(database.Pool)
	secretStore := db.NewSecretStore(database.Pool)
	connectorStore := db.NewConnectorStore(database.Pool)
	sessionStore := db.NewSessionStore(database.Pool)

	// Initialize encryption — auto-generates key on first run
	encryptor, err := crypto.LoadOrGenerateKey(bootstrap.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize encryption: %v", err)
	}
	log.Infof("Encryption key loaded from %s", bootstrap.EncryptionKey)

	// Create WebSocket hub
	hub := web.NewHub()
	go hub.Run()

	// Create run log registry and attach hook to standard logger
	runLogRegistry := web.NewRunLogRegistry()
	log.AddHook(web.NewRunLogHook(runLogRegistry, hub))

	// Create stores (continued)
	scheduleStore := db.NewScheduleStore(database.Pool)

	// Create services and handlers
	credResolver := credentials.NewResolver(connectorStore, secretStore, encryptor)
	exporter := web.NewResultExporter(connectorStore, credResolver)
	scenarioService := web.NewScenarioService(runStore, scenarioStore, packStore, configStore, credResolver, exporter, hub, runLogRegistry, bootstrap.DataDir)
	packHandlers := web.NewPackHandlers(packStore, bootstrap.DataDir)
	secretHandlers := web.NewSecretHandlers(secretStore, encryptor)
	connectorHandlers := web.NewConnectorHandlers(connectorStore, secretStore, scenarioStore, runStore, credResolver)

	// Create auth handlers
	authHandlers := auth.NewHandlers(sessionStore, auth.Config{
		GoogleClientID:     bootstrap.Auth.GoogleClientID,
		GoogleClientSecret: bootstrap.Auth.GoogleClientSecret,
		AllowedDomain:      bootstrap.Auth.AllowedDomain,
		SessionTTL:         bootstrap.Auth.SessionTTL,
	})

	// Create scheduler (before handlers, since handlers need scheduler reference)
	scheduler := web.NewScheduler(scheduleStore, scenarioStore, scenarioService)
	scheduleHandlers := web.NewScheduleHandlers(scheduleStore, scenarioStore, scheduler)
	handlers := web.NewHandlers(scenarioService, scenarioStore, runStore, configStore, scheduler, bootstrap.DataDir)
	if err := scheduler.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Server config
	serverCfg := &web.ServerConfig{
		Port:    bootstrap.WebPort,
		DevMode: bootstrap.DevMode,
	}

	// Create and start server
	server := web.NewServer(handlers, packHandlers, secretHandlers, scheduleHandlers, connectorHandlers, authHandlers, hub, serverCfg, sessionStore)
	log.Infof("Starting simrun-server on :%s", bootstrap.WebPort)

	// Cleanup expired auth sessions periodically
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := sessionStore.DeleteExpired(context.Background()); err != nil {
					log.Warnf("Failed to cleanup expired sessions: %v", err)
				}
			}
		}
	}()

	// Retention sweepers: expire run logs and whole assessments by age. Both run
	// once at startup and then hourly, re-reading AppConfig each tick so admin
	// changes apply without a restart.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		sweep := func() {
			cfg, err := configStore.GetAppConfig(context.Background())
			if err != nil {
				log.Warnf("Retention sweep: failed to load config: %v", err)
				return
			}
			web.SweepRunLogs(bootstrap.DataDir, cfg.AssessmentLogRetentionEnabled, cfg.AssessmentLogRetentionDays)
			web.SweepAssessments(context.Background(), runStore, bootstrap.DataDir, cfg.AssessmentRetentionEnabled, cfg.AssessmentRetentionDays)
		}
		sweep()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Info("Shutting down...")
}
