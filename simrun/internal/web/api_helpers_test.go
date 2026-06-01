package web_test

import "time"

// Eventually constants used across api_*_test.go files. Async run goroutines
// publish to the run store on the order of milliseconds; these bounds keep
// flakes near zero on CI while keeping the suite fast.
const (
	eventuallyTimeout = 2 * time.Second
	eventuallyTick    = 10 * time.Millisecond
)
