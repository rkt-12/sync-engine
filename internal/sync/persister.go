package sync

import (
	"context"

	"sync-engine/internal/crdt"
)

type Persister interface {
	// AppendOperation persists op and returns the sequence_id assigned to it
	AppendOperation(ctx context.Context, op crdt.Operation) (sequenceID int64, err error)

	// LoadOperationsAfter returns up to limit operations with sequence_id > afterSequence, in ascending order
	LoadOperationsAfter(ctx context.Context, documentID string, afterSequence int64, limit int) (ops []crdt.Operation, highestSequence int64, hasMore bool, err error)
}
