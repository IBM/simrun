package detonators

type Detonator interface {
	Detonate() (map[string]string, error)

	String() string

	// SimulationId returns the ID of the attack simulation being detonated
	SimulationId() string

	// CloudProvider returns the cloud provider targeted by this detonator.
	// Returns "aws", "gcp", "azure", or "" if unknown.
	CloudProvider() string

	// PackName returns the name of the pack for simrun detonators.
	// Returns empty string for non-simrun detonators.
	PackName() string

	// SetRunID sets the run ID for structured log routing.
	SetRunID(runID string)

	// SetStatusCallback sets a callback for reporting phase transitions during detonation.
	// Detonators should call this with phase names like "warmup" or "detonating".
	SetStatusCallback(callback func(phase string))

	// SetEnvVars sets run-specific environment variables for credential isolation.
	// These are used instead of the global process env when spawning child
	// processes or configuring SDK clients.
	SetEnvVars(envVars map[string]string)
}
