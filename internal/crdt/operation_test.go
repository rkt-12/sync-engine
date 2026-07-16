package crdt

import "testing"

func TestInsertOperation_ImplementsOperationInterface(t *testing.T) {
	op := InsertOperation{
		OperationID:     OperationID{ClientID: 1, Counter: 5},
		DocumentID:      "doc-1",
		ClientID:        1,
		LogicalClock:    5,
		ParentElementID: ElementID(RootID),
		ElementID:       ElementID{ClientID: 1, Counter: 5},
		Value:           'a',
	}

	var o Operation = op // compile-time check: InsertOperation satisfies Operation

	if o.ID() != op.OperationID {
		t.Errorf("ID() = %+v, want %+v", o.ID(), op.OperationID)
	}
	if o.Document() != "doc-1" {
		t.Errorf("Document() = %q, want %q", o.Document(), "doc-1")
	}
	if o.Kind() != KindInsert {
		t.Errorf("Kind() = %q, want %q", o.Kind(), KindInsert)
	}
}

func TestDeleteOperation_ImplementsOperationInterface(t *testing.T) {
	op := DeleteOperation{
		OperationID:     OperationID{ClientID: 2, Counter: 9},
		DocumentID:      "doc-1",
		ClientID:        2,
		LogicalClock:    9,
		TargetElementID: ElementID{ClientID: 1, Counter: 5},
	}

	var o Operation = op

	if o.ID() != op.OperationID {
		t.Errorf("ID() = %+v, want %+v", o.ID(), op.OperationID)
	}
	if o.Document() != "doc-1" {
		t.Errorf("Document() = %q, want %q", o.Document(), "doc-1")
	}
	if o.Kind() != KindDelete {
		t.Errorf("Kind() = %q, want %q", o.Kind(), KindDelete)
	}
}

func TestInsertOperation_OperationIDEqualsElementID_ByConvention(t *testing.T) {
	// This test documents the Phase 0 decision (docs/crdt-specification.md):
	// for inserts, OperationID and ElementID are numerically the same
	// value -- one causal event, one identity. It's the caller's
	// responsibility to construct them equal; this test exists so that
	// convention doesn't silently rot if someone refactors operation
	// construction later.
	id := Identifier{ClientID: 7, Counter: 42}
	op := InsertOperation{
		OperationID:     OperationID(id),
		ElementID:       ElementID(id),
		DocumentID:      "doc-1",
		ClientID:        7,
		LogicalClock:    42,
		ParentElementID: ElementID(RootID),
		Value:           'x',
	}

	if Identifier(op.OperationID) != Identifier(op.ElementID) {
		t.Errorf("expected OperationID == ElementID for an insert, got %+v vs %+v",
			op.OperationID, op.ElementID)
	}
}
