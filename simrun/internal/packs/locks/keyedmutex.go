// Package locks provides a process-global keyed mutex used to serialize
// mutating filesystem operations on a single pack's cache directory.
//
// The lock must live at package scope: the runner factory and resolver are
// constructed fresh per HTTP request and per detonation, so an
// instance-scoped lock would not be observed across those independent
// instances.
package locks

import "sync"

// mutexes holds one *sync.Mutex per key. It is never pruned: the number of
// distinct keys is bounded by the pack catalog size, so growth is negligible.
var mutexes sync.Map

// Acquire locks the mutex associated with key and returns a release closure
// that unlocks it. Calls with the same key are serialized; calls with
// different keys proceed concurrently.
func Acquire(key string) (release func()) {
	m, _ := mutexes.LoadOrStore(key, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
