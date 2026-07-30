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

	sequenceID, err := ops.AppendOperation(ctx, original)
	if err != nil {
		t.Fatalf("AppendOperation failed: %v", err)
	}
	if sequenceID <= 0 {
		t.Errorf("expected a positive sequence_id, got %d", sequenceID)
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

	insertSeq, err := ops.AppendOperation(ctx, insert)
	if err != nil {
		t.Fatalf("AppendOperation(insert) failed: %v", err)
	}
	deleteSeq, err := ops.AppendOperation(ctx, del)
	if err != nil {
		t.Fatalf("AppendOperation(delete) failed: %v", err)
	}
	if deleteSeq <= insertSeq {
		t.Errorf("expected delete's sequence_id (%d) to be greater than insert's (%d)", deleteSeq, insertSeq)
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

	var firstSequence int64
	for i := 0; i < 3; i++ {
		sequenceID, err := ops.AppendOperation(ctx, op)
		if err != nil {
			t.Fatalf("AppendOperation attempt %d failed: %v", i+1, err)
		}
		if i == 0 {
			firstSequence = sequenceID
			if firstSequence <= 0 {
				t.Fatalf("expected a positive sequence_id on first append, got %d", firstSequence)
			}
		} else if sequenceID != firstSequence {
			t.Errorf("attempt %d: expected the same sequence_id (%d) as the first append, got %d",
				i+1, firstSequence, sequenceID)
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
		if _, err := opStore.AppendOperation(ctx, op); err != nil {
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

func TestOperationStore_LoadOperationsAfter_Pagination(t *testing.T) {
	pool := testPool(t)
	docs := NewDocumentStore(pool)
	ops := NewOperationStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docID := testDocumentID(t)
	createTestDocument(t, ctx, docs, docID)

	root := crdt.ElementID(crdt.RootID)
	prev := root
	var sequences []int64
	for i := uint64(1); i <= 5; i++ {
		op := crdt.InsertOperation{
			OperationID: crdt.OperationID{ClientID: 1, Counter: i}, DocumentID: docID,
			ClientID: 1, LogicalClock: i, ParentElementID: prev,
			ElementID: crdt.ElementID{ClientID: 1, Counter: i}, Value: rune('a' + i - 1),
		}
		seq, err := ops.AppendOperation(ctx, op)
		if err != nil {
			t.Fatalf("AppendOperation %d failed: %v", i, err)
		}
		sequences = append(sequences, seq)
		prev = op.ElementID
	}

	// First page: 2 rows, starting from the beginning.
	page1, highest1, hasMore1, err := ops.LoadOperationsAfter(ctx, docID, 0, 2)
	if err != nil {
		t.Fatalf("LoadOperationsAfter (page 1) failed: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page 1: expected 2 operations, got %d", len(page1))
	}
	if !hasMore1 {
		t.Error("page 1: expected hasMore=true, got false")
	}
	if highest1 != sequences[1] {
		t.Errorf("page 1: highestSequence = %d, want %d", highest1, sequences[1])
	}

	// Second page: continue from where page 1 left off.
	page2, highest2, hasMore2, err := ops.LoadOperationsAfter(ctx, docID, highest1, 2)
	if err != nil {
		t.Fatalf("LoadOperationsAfter (page 2) failed: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page 2: expected 2 operations, got %d", len(page2))
	}
	if !hasMore2 {
		t.Error("page 2: expected hasMore=true, got false")
	}
	if highest2 != sequences[3] {
		t.Errorf("page 2: highestSequence = %d, want %d", highest2, sequences[3])
	}

	// Final page: the remainder, hasMore should now be false.
	page3, highest3, hasMore3, err := ops.LoadOperationsAfter(ctx, docID, highest2, 2)
	if err != nil {
		t.Fatalf("LoadOperationsAfter (page 3) failed: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("page 3: expected 1 operation, got %d", len(page3))
	}
	if hasMore3 {
		t.Error("page 3: expected hasMore=false, got true")
	}
	if highest3 != sequences[4] {
		t.Errorf("page 3: highestSequence = %d, want %d", highest3, sequences[4])
	}

	// Requesting after the very latest sequence should return nothing.
	page4, highest4, hasMore4, err := ops.LoadOperationsAfter(ctx, docID, highest3, 2)
	if err != nil {
		t.Fatalf("LoadOperationsAfter (page 4, empty) failed: %v", err)
	}
	if len(page4) != 0 || hasMore4 || highest4 != 0 {
		t.Errorf("expected an empty final page, got ops=%d hasMore=%v highest=%d", len(page4), hasMore4, highest4)
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
	if _, err := ops.AppendOperation(ctx, op); err == nil {
		t.Fatal("expected an error appending an operation for a nonexistent document, got nil")
	}
}
