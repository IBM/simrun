// Package envutil provides helpers for threaded environment variable management.
// Instead of mutating the global process environment with os.Setenv (which causes
// race conditions between concurrent runs), callers pass explicit env var maps
// through the execution chain. Each helper falls back to the process environment
// when the explicit map is nil (CLI path).
package envutil

import (
	"os"
	"strings"
)

// Lookup returns the value for key from envVars. When envVars is nil, falls
// back to os.Getenv(key) — preserves the legacy "no map = inherit process env"
// behavior. When envVars is non-nil, returns "" for absent keys (no fallback)
// so dev-shell process env can't leak into callers that pass an explicit map.
func Lookup(envVars map[string]string, key string) string {
	if envVars == nil {
		return os.Getenv(key)
	}
	return envVars[key]
}

// MergeWithProcessEnv returns a copy of os.Environ() with the provided
// env vars merged on top. If envVars is nil, returns os.Environ() as-is.
func MergeWithProcessEnv(envVars map[string]string) []string {
	if envVars == nil {
		return os.Environ()
	}

	base := make(map[string]string)
	for _, e := range os.Environ() {
		if i := strings.IndexByte(e, '='); i >= 0 {
			base[e[:i]] = e[i+1:]
		}
	}

	for k, v := range envVars {
		base[k] = v
	}

	result := make([]string, 0, len(base))
	for k, v := range base {
		result = append(result, k+"="+v)
	}
	return result
}
