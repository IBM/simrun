package pack

import (
	"context"
	"fmt"
	"time"
)

// Wait sleeps for the specified duration, respecting context cancellation.
func Wait(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitFor polls the condition function until it returns true or the timeout is reached.
func WaitFor(ctx context.Context, interval, timeout time.Duration, condition func() bool) error {
	deadline := time.Now().Add(timeout)

	if condition() {
		return nil
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if condition() {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("wait condition not met after %s", timeout)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// GetString safely extracts a string value from a map[string]any.
func GetString(m map[string]any, key string) (string, bool) {
	val, ok := m[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt safely extracts an integer value from a map[string]any.
// Handles both int and float64 (common when unmarshaling JSON).
func GetInt(m map[string]any, key string) (int, bool) {
	val, ok := m[key]
	if !ok {
		return 0, false
	}

	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// RequireString extracts a required string from a map[string]any and returns an error Result
// if missing or empty. Useful for extracting values from input.Params.
//
// Example:
//
//	region, errResult := pack.RequireString(input.Params, "region")
//	if errResult != nil {
//	    return errResult, nil
//	}
func RequireString(m map[string]any, key string) (string, *Result) {
	val, ok := GetString(m, key)
	if !ok || val == "" {
		return "", ErrorResult(ErrCodeInvalidParams, fmt.Sprintf("missing required parameter: %s", key))
	}
	return val, nil
}

