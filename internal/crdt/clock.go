// This file implements a Lamport Logical Clock, which is a mechanism
// used in distributed systems to order events without relying on physical time.
package crdt

import "sync"

type LogicalClock struct {
	mu      sync.Mutex
	counter uint64
}

func NewLogicalClock() *LogicalClock {
	return &LogicalClock{}
}

// Next increments the clock and returns the new value.
func (c *LogicalClock) Next() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counter++
	return c.counter
}

// Observe updates the clock on receipt of a remote operation's logical clock value, implementing
// "receive event -> local = max(local, remote)".
func (c *LogicalClock) Observe(remote uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if remote > c.counter {
		c.counter = remote
	}
}

// Current returns the clock's current value without advancing it.
func (c *LogicalClock) Current() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counter
}
