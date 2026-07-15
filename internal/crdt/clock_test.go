package crdt

import (
	"sync"
	"testing"
)

func TestLogicalClock_NextIncrements(t *testing.T) {
	c := NewLogicalClock()

	if got := c.Next(); got != 1 {
		t.Errorf("first Next() = %d, want 1", got)
	}
	if got := c.Next(); got != 2 {
		t.Errorf("second Next() = %d, want 2", got)
	}
	if got := c.Current(); got != 2 {
		t.Errorf("Current() = %d, want 2", got)
	}
}

func TestLogicalClock_ObserveAdvancesToMax(t *testing.T) {
	c := NewLogicalClock()
	c.Next() // counter = 1

	c.Observe(10)
	if got := c.Current(); got != 10 {
		t.Errorf("after Observe(10), Current() = %d, want 10", got)
	}

	// Observing a smaller value must never move the clock backward.
	c.Observe(3)
	if got := c.Current(); got != 10 {
		t.Errorf("after Observe(3) on a clock at 10, Current() = %d, want 10 (must not decrease)", got)
	}
}

func TestLogicalClock_NextAfterObserveContinuesForward(t *testing.T) {
	c := NewLogicalClock()
	c.Observe(100)

	if got := c.Next(); got != 101 {
		t.Errorf("Next() after Observe(100) = %d, want 101", got)
	}
}

// TestLogicalClock_ConcurrentNext verifies the clock is safe under
// concurrent access and that every value it hands out is unique
// Run with -race to actually verify the safety property, not just the count.
func TestLogicalClock_ConcurrentNext(t *testing.T) {
	c := NewLogicalClock()

	const goroutines = 50
	const perGoroutine = 200

	results := make(chan uint64, goroutines*perGoroutine)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				results <- c.Next()
			}
		}()
	}

	wg.Wait()
	close(results)

	seen := make(map[uint64]bool, goroutines*perGoroutine)
	for v := range results {
		if seen[v] {
			t.Fatalf("duplicate value %d returned by Next() under concurrent access", v)
		}
		seen[v] = true
	}

	if len(seen) != goroutines*perGoroutine {
		t.Errorf("got %d unique values, want %d", len(seen), goroutines*perGoroutine)
	}
}
