package collectors

import (
	"context"
)

// Collector is an interface that every collector should implement to collect logs
// produced by attack activity for further analysis
type Collector interface {
	// Collect searches for logs matching the configured query and indicators,
	// and writes them to the output file. This is called once at the end of
	// scenario execution (when expectations pass or timeout is reached).
	// It returns the number of documents collected and any error that occurred.
	Collect(ctx context.Context, indicators map[string]string) (int, error)

	// String returns the textual, user-friendly representation of the collector
	String() string

	// GetOutputPath returns the path where the collected logs are stored
	GetOutputPath() string
}
