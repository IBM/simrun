package config

// AppConfig holds operational defaults that admins tune via the UI. Stored
// in the app_config table (one JSON value per key). Read via db.ConfigStore.
//
// Fields here are intentionally narrow: anything that varies per-target
// belongs in connectors, anything secret belongs in secret_groups, anything
// set at deploy belongs in Bootstrap.
type AppConfig struct {
	Parallelism                   int    `json:"parallelism"`
	TerraformVersion              string `json:"terraform_version"`
	PackLogsEnabled               bool   `json:"pack_logs_enabled"`
	SSHLoggingEnabled             bool   `json:"ssh_logging_enabled"`
	AssessmentLogRetentionEnabled bool   `json:"assessment_log_retention_enabled"`
	AssessmentLogRetentionDays    int    `json:"assessment_log_retention_days"`
	AssessmentRetentionEnabled    bool   `json:"assessment_retention_enabled"`
	AssessmentRetentionDays       int    `json:"assessment_retention_days"`
}

// DefaultAppConfig returns the default values used when no row exists for
// a key. Keep these aligned with the migration that backfills app_config.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		Parallelism:                   5,
		TerraformVersion:              "",
		PackLogsEnabled:               true,
		SSHLoggingEnabled:             false,
		AssessmentLogRetentionEnabled: true,
		AssessmentLogRetentionDays:    7,
		AssessmentRetentionEnabled:    false,
		AssessmentRetentionDays:       30,
	}
}
