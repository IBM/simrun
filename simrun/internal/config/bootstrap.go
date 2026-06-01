package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Bootstrap holds operator-set, deploy-time configuration loaded from environment
// variables. It is immutable for the lifetime of the process and is passed by
// dependency injection from main.go to services that need it.
//
// Bootstrap is the ONLY surface that reads SR_* env vars in v4. Everything else
// (Elastic/Datadog/AWS/GCP/Azure/K8s/SSH credentials, parallelism, terraform
// version, pack-logs and ssh-logs toggles) lives in the database.
type Bootstrap struct {
	DatabaseURL   string
	WebPort       string
	DataDir       string
	Debug         bool
	EncryptionKey string
	DevMode       bool
	WebURL        string
	Auth          AuthBootstrap
}

// AuthBootstrap holds Google OAuth bootstrap config. The client secret is a
// credential and is read from env only.
type AuthBootstrap struct {
	GoogleClientID     string
	GoogleClientSecret string
	AllowedDomain      string
	SessionTTL         time.Duration
}

// LoadBootstrap reads the bootstrap surface from environment variables.
// Returns an error if SR_DATABASE_URL is missing.
func LoadBootstrap() (*Bootstrap, error) {
	dbURL := os.Getenv("SR_DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("SR_DATABASE_URL is required")
	}

	dataDir := os.Getenv("SR_DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			dataDir = "."
		} else {
			dataDir = filepath.Join(home, ".simrun")
		}
	}

	port := os.Getenv("SR_WEB_PORT")
	if port == "" {
		port = "8080"
	}

	encKey := os.Getenv("SR_ENCRYPTION_KEY_FILE")
	if encKey == "" {
		encKey = filepath.Join(dataDir, "encryption.key")
	}

	sessionTTLHours := 168 // 7 days default
	if v := os.Getenv("SR_AUTH_SESSION_TTL_HOURS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			sessionTTLHours = parsed
		}
	}

	return &Bootstrap{
		DatabaseURL:   dbURL,
		WebPort:       port,
		DataDir:       dataDir,
		Debug:         os.Getenv("SR_DEBUG") != "" && os.Getenv("SR_DEBUG") != "0" && os.Getenv("SR_DEBUG") != "false",
		EncryptionKey: encKey,
		DevMode:       os.Getenv("SR_WEB_DEV") == "1",
		WebURL:        os.Getenv("SR_WEB_URL"),
		Auth: AuthBootstrap{
			GoogleClientID:     os.Getenv("SR_GOOGLE_CLIENT_ID"),
			GoogleClientSecret: os.Getenv("SR_GOOGLE_CLIENT_SECRET"),
			AllowedDomain:      os.Getenv("SR_GOOGLE_ALLOWED_DOMAIN"),
			SessionTTL:         time.Duration(sessionTTLHours) * time.Hour,
		},
	}, nil
}
