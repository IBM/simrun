// Package results provides a parallel scenario executor over the shared
// runner.ScenarioResult / runner.RunResult result types.
package results

// RunResult and ScenarioResult are defined in the runner package (the single
// in-memory result types shared across runner, executor, and web). This file is
// retained as the package anchor; see executor.go for the fan-out.
