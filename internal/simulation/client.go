package simulation

import (
	"fmt"
	"math/rand"

	"sync-engine/internal/crdt"
)

// Client simulates a collaborative editor. It holds a full local CRDT replica and its own logical clock.
type Client struct {
	ID      uint64
	Clock   *crdt.LogicalClock
	Replica *crdt.Document
}

func NewClient(id uint64, documentID string) *Client {
	return &Client{
		ID:      id,
		Clock:   crdt.NewLogicalClock(),
		Replica: crdt.NewDocument(documentID),
	}
}

// GenerateInsert simulates typing one random character at a random position in this client's current view of the document.
func (c *Client) GenerateInsert(rng *rand.Rand, alphabet string) crdt.InsertOperation {
	visible := c.Replica.VisibleSequence()
	parent := crdt.ElementID(crdt.RootID)
	if len(visible) > 0 {
		parent = visible[rng.Intn(len(visible))]
	}

	counter := c.Clock.Next()
	id := crdt.Identifier{ClientID: c.ID, Counter: counter}
	op := crdt.InsertOperation{
		OperationID:     crdt.OperationID(id),
		DocumentID:      c.Replica.ID(),
		ClientID:        c.ID,
		LogicalClock:    counter,
		ParentElementID: parent,
		ElementID:       crdt.ElementID(id),
		Value:           rune(alphabet[rng.Intn(len(alphabet))]),
	}

	if err := c.Replica.Insert(op); err != nil {
		// a failure here means the simulator has a bug, not the scenario
		// under test, so we fail loudly and immediately rather than
		// letting a corrupted run silently continue.
		panic(fmt.Sprintf("simulation: local Insert failed for generating client %d: %v", c.ID, err))
	}
	return op
}

// GenerateDelete simulates deleting a random visible character in this
// client's current view. ok is false if the client's view is currently
// empty, meaning there is nothing to delete.
func (c *Client) GenerateDelete(rng *rand.Rand) (op crdt.DeleteOperation, ok bool) {
	visible := c.Replica.VisibleSequence()
	if len(visible) == 0 {
		return crdt.DeleteOperation{}, false
	}

	target := visible[rng.Intn(len(visible))]
	counter := c.Clock.Next()
	op = crdt.DeleteOperation{
		OperationID:     crdt.OperationID{ClientID: c.ID, Counter: counter},
		DocumentID:      c.Replica.ID(),
		ClientID:        c.ID,
		LogicalClock:    counter,
		TargetElementID: target,
	}

	if err := c.Replica.Delete(op); err != nil {
		panic(fmt.Sprintf("simulation: local Delete failed for generating client %d: %v", c.ID, err))
	}
	return op, true
}

// Deliver applies a remote operation to this client's replica
func (c *Client) Deliver(op crdt.Operation) error {
	c.Clock.Observe(logicalClockOf(op))
	return c.Replica.Apply(op)
}

func logicalClockOf(op crdt.Operation) uint64 {
	switch o := op.(type) {
	case crdt.InsertOperation:
		return o.LogicalClock
	case crdt.DeleteOperation:
		return o.LogicalClock
	default:
		return 0
	}
}
