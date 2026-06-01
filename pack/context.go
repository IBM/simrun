package pack

import (
	"context"
	"fmt"

	"github.com/IBM/simrun/internal/version"
)

type contextKey int

const executionIDKey contextKey = iota

// WithExecutionID returns a context with the execution ID stored.
func WithExecutionID(ctx context.Context, executionID string) context.Context {
	return context.WithValue(ctx, executionIDKey, executionID)
}

// ExecutionIDFromContext retrieves the execution ID from context.
// Returns empty string if not found.
func ExecutionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(executionIDKey).(string); ok {
		return v
	}
	return ""
}

// UserAgent returns the formatted User-Agent string for the given execution ID.
// Format: "simrun/<version> (<execution_id>)"
// Returns empty string if executionID is empty.
func UserAgent(executionID string) string {
	if executionID == "" {
		return ""
	}
	return fmt.Sprintf("simrun/%s (%s)", version.Version, executionID)
}
