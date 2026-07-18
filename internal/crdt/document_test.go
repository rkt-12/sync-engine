package crdt

import "testing"

// helper: build an InsertOperation where OperationID == ElementID, per
// the Phase 0 convention documented in operation.go.
func insertOp(docID string, clientID, counter uint64, parent ElementID, value rune) InsertOperation {
	id := Identifier{ClientID: clientID, Counter: counter}
	return InsertOperation{
		OperationID:     OperationID(id),
		DocumentID:      docID,
		ClientID:        clientID,
		LogicalClock:    counter,
		ParentElementID: parent,
		ElementID:       ElementID(id),
		Value:           value,
	}
}

func deleteOp(docID string, clientID, counter uint64, target ElementID) DeleteOperation {
	return DeleteOperation{
		OperationID:     OperationID{ClientID: clientID, Counter: counter},
		DocumentID:      docID,
		ClientID:        clientID,
		LogicalClock:    counter,
		TargetElementID: target,
	}
}

func TestDocument_InsertSingleCharacter(t *testing.T) {
	d := NewDocument("doc-1")
	op := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')

	if err := d.Insert(op); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if got := d.Materialize(); got != "a" {
		t.Errorf("Materialize() = %q, want %q", got, "a")
	}
}

func TestDocument_InsertSequentialChain(t *testing.T) {
	// a -> b -> c, each inserted after the previous: "abc"
	d := NewDocument("doc-1")
	a := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')
	b := insertOp("doc-1", 1, 2, a.ElementID, 'b')
	c := insertOp("doc-1", 1, 3, b.ElementID, 'c')

	for _, op := range []InsertOperation{a, b, c} {
		if err := d.Insert(op); err != nil {
			t.Fatalf("Insert(%+v) failed: %v", op, err)
		}
	}

	if got := d.Materialize(); got != "abc" {
		t.Errorf("Materialize() = %q, want %q", got, "abc")
	}
}

func TestDocument_Insert_UnknownParent_ReturnsError(t *testing.T) {
	d := NewDocument("doc-1")
	ghost := ElementID{ClientID: 99, Counter: 99}
	op := insertOp("doc-1", 1, 1, ghost, 'a')

	err := d.Insert(op)
	if err == nil {
		t.Fatal("expected an error inserting after an unknown parent, got nil")
	}
}

func TestDocument_Insert_WrongDocumentID_ReturnsError(t *testing.T) {
	d := NewDocument("doc-1")
	op := insertOp("doc-2", 1, 1, ElementID(RootID), 'a')

	err := d.Insert(op)
	if err == nil {
		t.Fatal("expected an error for mismatched document ID, got nil")
	}
}

func TestDocument_Insert_Idempotent(t *testing.T) {
	d := NewDocument("doc-1")
	op := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')

	if err := d.Insert(op); err != nil {
		t.Fatalf("first Insert failed: %v", err)
	}
	if err := d.Insert(op); err != nil {
		t.Fatalf("second (duplicate) Insert should be a no-op, got error: %v", err)
	}
	if got := d.Materialize(); got != "a" {
		t.Errorf("Materialize() after duplicate insert = %q, want %q (no double character)", got, "a")
	}
}

func TestDocument_Delete_TombstonesExistingElement(t *testing.T) {
	d := NewDocument("doc-1")
	a := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')
	if err := d.Insert(a); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	del := deleteOp("doc-1", 1, 2, a.ElementID)
	if err := d.Delete(del); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if got := d.Materialize(); got != "" {
		t.Errorf("Materialize() after delete = %q, want empty string", got)
	}
}

func TestDocument_Delete_Idempotent(t *testing.T) {
	d := NewDocument("doc-1")
	a := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')
	d.Insert(a)

	del := deleteOp("doc-1", 1, 2, a.ElementID)
	if err := d.Delete(del); err != nil {
		t.Fatalf("first Delete failed: %v", err)
	}
	if err := d.Delete(del); err != nil {
		t.Fatalf("second (duplicate) Delete should be a no-op, got error: %v", err)
	}
	if got := d.Materialize(); got != "" {
		t.Errorf("Materialize() = %q, want empty string", got)
	}
}

func TestDocument_Delete_WrongDocumentID_ReturnsError(t *testing.T) {
	d := NewDocument("doc-1")
	op := deleteOp("doc-2", 1, 1, ElementID{ClientID: 1, Counter: 1})

	if err := d.Delete(op); err == nil {
		t.Fatal("expected an error for mismatched document ID, got nil")
	}
}

func TestDocument_DeleteBeforeInsert(t *testing.T) {
	d := NewDocument("doc-1")
	a := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')

	// The delete arrives first, targeting an element that doesn't exist yet.
	del := deleteOp("doc-1", 2, 1, a.ElementID)
	if err := d.Delete(del); err != nil {
		t.Fatalf("Delete (before insert) should be accepted as pending, got error: %v", err)
	}

	// Document is still empty -- nothing to delete yet.
	if got := d.Materialize(); got != "" {
		t.Errorf("Materialize() before insert arrives = %q, want empty string", got)
	}

	// Now the insert arrives.
	if err := d.Insert(a); err != nil {
		t.Fatalf("Insert (after pending delete) failed: %v", err)
	}

	// The element should be immediately tombstoned upon insertion.
	if got := d.Materialize(); got != "" {
		t.Errorf("Materialize() after insert = %q, want empty string (should be pre-tombstoned)", got)
	}
}

func TestDocument_DeleteBeforeInsert_MatchesInsertBeforeDelete(t *testing.T) {
	a := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')
	del := deleteOp("doc-1", 2, 1, a.ElementID)

	// Replica 1: insert then delete.
	d1 := NewDocument("doc-1")
	d1.Insert(a)
	d1.Delete(del)

	// Replica 2: delete then insert (delete-before-insert).
	d2 := NewDocument("doc-1")
	d2.Delete(del)
	d2.Insert(a)

	if d1.Materialize() != d2.Materialize() {
		t.Errorf("order of insert/delete changed the result: d1=%q d2=%q",
			d1.Materialize(), d2.Materialize())
	}
}

func TestDocument_ConcurrentInsertion_SameParent_OrderIndependentConvergence(t *testing.T) {
	// Base document: "a". Two clients concurrently insert immediately
	// after 'a': X from client 2, Y from client 3. Neither has seen the
	// other's operation when generating it -- both are siblings of each
	// other under parent 'a', and nothing else is competing for that
	// position, so this isolates the two-way tie-break cleanly.
	a := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')
	x := insertOp("doc-1", 2, 1, a.ElementID, 'X')
	y := insertOp("doc-1", 3, 1, a.ElementID, 'Y')

	// Hand-derived expected order per docs/crdt-specification.md: siblings
	// are ordered descending by Identifier. x=(client 2, counter 1),
	// y=(client 3, counter 1) -- equal counters, tie-break on ClientID:
	// y's ClientID (3) > x's ClientID (2), so y sorts before x.
	const want = "aYX"

	// Replica 1 applies X before Y.
	d1 := NewDocument("doc-1")
	d1.Insert(a)
	d1.Insert(x)
	d1.Insert(y)

	// Replica 2 applies Y before X.
	d2 := NewDocument("doc-1")
	d2.Insert(a)
	d2.Insert(y)
	d2.Insert(x)

	if d1.Materialize() != d2.Materialize() {
		t.Fatalf("concurrent insertion order affected convergence: d1=%q d2=%q",
			d1.Materialize(), d2.Materialize())
	}
	if got := d1.Materialize(); got != want {
		t.Errorf("Materialize() = %q, want %q", got, want)
	}
}

func TestDocument_ApplyBatch_OrderIndependence_LargerScenario(t *testing.T) {
	root := ElementID(RootID)
	a := insertOp("doc-1", 1, 1, root, 'a')
	b := insertOp("doc-1", 1, 2, a.ElementID, 'b')
	c := insertOp("doc-1", 1, 3, b.ElementID, 'c')
	x := insertOp("doc-1", 2, 1, a.ElementID, 'X') // concurrent w/ b, sibling of a
	delB := deleteOp("doc-1", 3, 1, b.ElementID)   // deletes 'b'

	forward := []Operation{a, b, c, x, delB}
	// Reordered, but NOT arbitrarily: 'a' must still precede 'b' (its
	// parent), and 'b' must still precede 'c' (its parent) -- inserts are
	// assumed causally delivered relative to their own parent (see the
	// Phase 2 design note in docs/crdt-specification.md and
	// ErrParentNotFound). What IS legitimately reordered: delB moves
	// before its target 'b' entirely (explicitly supported --
	// delete-before-insert), and 'x' (concurrent with b/c, a sibling of b
	// under 'a') moves earlier since it has no causal dependency on them.
	reverse := []Operation{delB, a, x, b, c}

	d1 := NewDocument("doc-1")
	if err := d1.ApplyBatch(forward); err != nil {
		t.Fatalf("ApplyBatch(forward) failed: %v", err)
	}

	d2 := NewDocument("doc-1")
	if err := d2.ApplyBatch(reverse); err != nil {
		t.Fatalf("ApplyBatch(reverse) failed: %v", err)
	}

	if d1.Materialize() != d2.Materialize() {
		t.Fatalf("differing application order produced different results: forward=%q reverse=%q",
			d1.Materialize(), d2.Materialize())
	}
	t.Logf("converged result: %q", d1.Materialize())
}

func TestDocument_ApplyBatch_DuplicateOperationsAreHarmless(t *testing.T) {
	a := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')
	b := insertOp("doc-1", 1, 2, a.ElementID, 'b')

	ops := []Operation{a, b, a, a, b} // same operations repeated

	d := NewDocument("doc-1")
	if err := d.ApplyBatch(ops); err != nil {
		t.Fatalf("ApplyBatch with duplicates failed: %v", err)
	}
	if got := d.Materialize(); got != "ab" {
		t.Errorf("Materialize() = %q, want %q (duplicates must not appear)", got, "ab")
	}
}

func TestDocument_Apply_UnsupportedOperationType(t *testing.T) {
	d := NewDocument("doc-1")

	err := d.Apply(fakeOperation{})
	if err == nil {
		t.Fatal("expected an error for an unsupported Operation implementation, got nil")
	}
}

func TestDocument_HasOperation(t *testing.T) {
	d := NewDocument("doc-1")
	op := insertOp("doc-1", 1, 1, ElementID(RootID), 'a')

	if d.HasOperation(OperationID(op.OperationID)) {
		t.Fatal("HasOperation should be false before Insert is called")
	}
	d.Insert(op)
	if !d.HasOperation(OperationID(op.OperationID)) {
		t.Fatal("HasOperation should be true after Insert is called")
	}
}

// fakeOperation exists only to exercise Apply's default case: an
// Operation implementation that is neither InsertOperation nor
// DeleteOperation.
type fakeOperation struct{}

func (fakeOperation) ID() OperationID     { return OperationID{} }
func (fakeOperation) Document() string    { return "doc-1" }
func (fakeOperation) Kind() OperationKind { return "fake" }
