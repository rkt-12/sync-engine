package store

import (
	"context"
	"testing"
	"time"

	"sync-engine/internal/crdt"
)

// createTestDocument creates a document row so operations referencing it
// satisfy the FOREIGN KEY constraint (operations.document_id ->
// documents.id).
func createTestDocument(t *testing.T, ctx context.Context, docs *DocumentStore, id string) {
	t.Helper()
	if err := docs.CreateDocument(ctx, id, "Test Document"); err != nil {
		t.Fatalf("createTestDocument: %v", err)
	}
}

func TestOperationStore_AppendAndLoad_InsertRoundTrip(t *testing.T) {
	pool := testPool(t)
	docs := NewDocumentStore(pool)
	ops := NewOperationStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docID := testDocumentID(t)
	createTestDocument(t, ctx, docs, docID)

	original := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 1, Counter: 1},
		DocumentID:      docID,
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: crdt.ElementID(crdt.RootID),
		ElementID:       crdt.ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}

	if err := ops.AppendOperation(ctx, original); err != nil {
		t.Fatalf("AppendOperation failed: %v", err)
	}

	loaded, err := ops.LoadOperations(ctx, docID)
	if err != nil {
		t.Fatalf("LoadOperations failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 loaded operation, got %d", len(loaded))
	}

	got, ok := loaded[0].(crdt.InsertOperation)
	if !ok {
		t.Fatalf("expected loaded operation to be InsertOperation, got %T", loaded[0])
	}
	if got != original {
		t.Errorf("round trip mismatch:\n  original = %+v\n  got      = %+v", original, got)
	}
}

func TestOperationStore_AppendAndLoad_DeleteRoundTrip(t *testing.T) {
	pool := testPool(t)
	docs := NewDocumentStore(pool)
	ops := NewOperationStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docID := testDocumentID(t)
	createTestDocument(t, ctx, docs, docID)

	insert := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 1, Counter: 1},
		DocumentID:      docID,
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: crdt.ElementID(crdt.RootID),
		ElementID:       crdt.ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}
	del := crdt.DeleteOperation{
		OperationID:     crdt.OperationID{ClientID: 2, Counter: 1},
		DocumentID:      docID,
		ClientID:        2,
		LogicalClock:    2,
		TargetElementID: insert.ElementID,
	}

	if err := ops.AppendOperation(ctx, insert); err != nil {
		t.Fatalf("AppendOperation(insert) failed: %v", err)
	}
	if err := ops.AppendOperation(ctx, del); err != nil {
		t.Fatalf("AppendOperation(delete) failed: %v", err)
	}

	loaded, err := ops.LoadOperations(ctx, docID)
	if err != nil {
		t.Fatalf("LoadOperations failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded operations, got %d", len(loaded))
	}

	gotDelete, ok := loaded[1].(crdt.DeleteOperation)
	if !ok {
		t.Fatalf("expected second loaded operation to be DeleteOperation, got %T", loaded[1])
	}
	if gotDelete != del {
		t.Errorf("delete round trip mismatch:\n  original = %+v\n  got      = %+v", del, gotDelete)
	}
}

func TestOperationStore_AppendOperation_IdempotentAtDatabaseLayer(t *testing.T) {
	pool := testPool(t)
	docs := NewDocumentStore(pool)
	ops := NewOperationStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docID := testDocumentID(t)
	createTestDocument(t, ctx, docs, docID)

	op := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 1, Counter: 1},
		DocumentID:      docID,
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: crdt.ElementID(crdt.RootID),
		ElementID:       crdt.ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}

	for i := 0; i < 3; i++ {
		if err := ops.AppendOperation(ctx, op); err != nil {
			t.Fatalf("AppendOperation attempt %d failed: %v", i+1, err)
		}
	}

	loaded, err := ops.LoadOperations(ctx, docID)
	if err != nil {
		t.Fatalf("LoadOperations failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected exactly 1 row despite 3 AppendOperation calls with the same operation_id, got %d", len(loaded))
	}
}

// TestOperationStore_Replay_MatchesInMemoryApplication is the key
// correctness test for this phase: it builds a small scenario (including
// a concurrent insert and a delete-before-insert case), applies it
// directly in memory via crdt.Document, and separately persists +
// replays the same operations through Postgres -- then asserts both
// produce the identical materialized document. This is
// docs/invariants.md Invariant 8's replay half in miniature (full
// snapshot+replay equivalence is Phase 8; this is "replay alone
// reproduces the same state the operations describe").
func TestOperationStore_Replay_MatchesInMemoryApplication(t *testing.T) {
	pool := testPool(t)
	docs := NewDocumentStore(pool)
	opStore := NewOperationStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docID := testDocumentID(t)
	createTestDocument(t, ctx, docs, docID)

	root := crdt.ElementID(crdt.RootID)
	a := crdt.InsertOperation{
		OperationID: crdt.OperationID{ClientID: 1, Counter: 1}, DocumentID: docID,
		ClientID: 1, LogicalClock: 1, ParentElementID: root,
		ElementID: crdt.ElementID{ClientID: 1, Counter: 1}, Value: 'a',
	}
	b := crdt.InsertOperation{
		OperationID: crdt.OperationID{ClientID: 1, Counter: 2}, DocumentID: docID,
		ClientID: 1, LogicalClock: 2, ParentElementID: a.ElementID,
		ElementID: crdt.ElementID{ClientID: 1, Counter: 2}, Value: 'b',
	}
	x := crdt.InsertOperation{ // concurrent sibling of b, from a different client
		OperationID: crdt.OperationID{ClientID: 2, Counter: 1}, DocumentID: docID,
		ClientID: 2, LogicalClock: 1, ParentElementID: a.ElementID,
		ElementID: crdt.ElementID{ClientID: 2, Counter: 1}, Value: 'X',
	}
	delB := crdt.DeleteOperation{
		OperationID: crdt.OperationID{ClientID: 3, Counter: 1}, DocumentID: docID,
		ClientID: 3, LogicalClock: 1, TargetElementID: b.ElementID,
	}

	ops := []crdt.Operation{a, b, x, delB}

	// Reference: apply directly in memory.
	reference := crdt.NewDocument(docID)
	if err := reference.ApplyBatch(ops); err != nil {
		t.Fatalf("in-memory ApplyBatch failed: %v", err)
	}

	// Persist the same operations.
	for _, op := range ops {
		if err := opStore.AppendOperation(ctx, op); err != nil {
			t.Fatalf("AppendOperation(%+v) failed: %v", op, err)
		}
	}

	// Replay purely from storage.
	replayed, err := opStore.Replay(ctx, docID)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if replayed.Materialize() != reference.Materialize() {
		t.Fatalf("replay diverged from in-memory application:\n  in-memory = %q\n  replayed  = %q",
			reference.Materialize(), replayed.Materialize())
	}
}

func TestOperationStore_LoadOperations_EmptyDocument(t *testing.T) {
	pool := testPool(t)
	docs := NewDocumentStore(pool)
	ops := NewOperationStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docID := testDocumentID(t)
	createTestDocument(t, ctx, docs, docID)

	loaded, err := ops.LoadOperations(ctx, docID)
	if err != nil {
		t.Fatalf("LoadOperations on empty document failed: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 operations for a fresh document, got %d", len(loaded))
	}
}

func TestOperationStore_AppendOperation_UnknownDocument_Errors(t *testing.T) {
	pool := testPool(t)
	ops := NewOperationStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	op := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 1, Counter: 1},
		DocumentID:      "this-document-was-never-created",
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: crdt.ElementID(crdt.RootID),
		ElementID:       crdt.ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}

	// Expected to fail on the FOREIGN KEY constraint (operations.document_id
	// -> documents.id) -- persisting an operation for a document that was
	// never created should not silently succeed.
	if err := ops.AppendOperation(ctx, op); err == nil {
		t.Fatal("expected an error appending an operation for a nonexistent document, got nil")
	}
}
