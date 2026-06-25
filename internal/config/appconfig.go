package config

// AppConfig holds operational defaults that admins tune via the UI. Stored
// in the app_config table (one JSON value per key). Read via db.ConfigStore.
//
// Fields here are intentionally narrow: anything that varies per-target
// belongs in connectors, anything secret belongs in secret_groups, anything
// set at deploy belongs in Bootstrap.
type AppConfig struct {
	Parallelism            int    `json:"parallelism"`
	TerraformVersion       string `json:"terraform_version"`
	PackLogsEnabled        bool   `json:"pack_logs_enabled"`
	SSHLoggingEnabled      bool   `json:"ssh_logging_enabled"`
	RunLogRetentionEnabled bool   `json:"run_log_retention_enabled"`
	RunLogRetentionDays    int    `json:"run_log_retention_days"`
	RunRetentionEnabled    bool   `json:"run_retention_enabled"`
	RunRetentionDays       int    `json:"run_retention_days"`
}

// DefaultAppConfig returns the default values used when no row exists for
// a key. Keep these aligned with the migration that backfills app_config.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		Parallelism:            5,
		TerraformVersion:       "",
		PackLogsEnabled:        true,
		SSHLoggingEnabled:      false,
		RunLogRetentionEnabled: true,
		RunLogRetentionDays:    7,
		RunRetentionEnabled:    false,
		RunRetentionDays:       30,
	}
}
