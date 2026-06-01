package locks

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAcquireSameKeySerializes verifies that two goroutines acquiring the same
// key never overlap inside the critical section. This is the core guarantee:
// concurrent mutating operations on one pack must not interleave and corrupt
// its cache directory.
func TestAcquireSameKeySerializes(t *testing.T) {
	const key = "pack-a"
	var inside int32
	var maxConcurrent int32

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := Acquire(key)
			defer release()

			n := atomic.AddInt32(&inside, 1)
			// Track the highest observed concurrency inside the section.
			for {
				m := atomic.LoadInt32(&maxConcurrent)
				if n <= m || atomic.CompareAndSwapInt32(&maxConcurrent, m, n) {
					break
				}
			}
			// Hold briefly to widen the window for an overlap to be observed.
			time.Sleep(time.Millisecond)
			atomic.AddInt32(&inside, -1)
		}()
	}
	wg.Wait()

	if maxConcurrent != 1 {
		t.Fatalf("same key allowed %d goroutines inside the critical section; want 1", maxConcurrent)
	}
}

// TestAcquireDifferentKeysConcurrent verifies that distinct keys do not block
// each other: operations on different packs must remain fully parallel.
func TestAcquireDifferentKeysConcurrent(t *testing.T) {
	const n = 8
	started := make(chan struct{}, n)
	proceed := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		key := string(rune('a' + i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := Acquire(key)
			defer release()
			started <- struct{}{}
			<-proceed // hold the lock until all goroutines are inside
		}()
	}

	// All n goroutines must reach inside their critical sections at once.
	// If different keys serialized, only one would start and this would block.
	for i := 0; i < n; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d/%d goroutines entered; different keys appear to block each other", i, n)
		}
	}
	close(proceed)
	wg.Wait()
}

// TestReleaseUnlocks verifies that release() frees the lock so a subsequent
// Acquire of the same key proceeds rather than deadlocking.
func TestReleaseUnlocks(t *testing.T) {
	const key = "pack-release"

	release := Acquire(key)
	release()

	done := make(chan struct{})
	go func() {
		release2 := Acquire(key)
		release2()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second Acquire blocked; release() did not unlock the mutex")
	}
}
