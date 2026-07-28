package protocol

import (
	"testing"

	"sync-engine/internal/crdt"
)

func TestEncodeDecode_Envelope_RoundTrip(t *testing.T) {
	original := Envelope{
		Type:            TypePing,
		ProtocolVersion: CurrentProtocolVersion,
	}

	data, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Type != original.Type || decoded.ProtocolVersion != original.ProtocolVersion {
		t.Errorf("round trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestDecode_InvalidJSON_Errors(t *testing.T) {
	_, err := Decode([]byte("not json"))
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestDecode_RejectsInvalidEnvelope(t *testing.T) {
	// Valid JSON, but fails ValidateEnvelope (missing clientId for
	// join_document).
	raw := []byte(`{"type":"join_document","protocolVersion":1,"documentId":"doc-1"}`)
	_, err := Decode(raw)
	if err == nil {
		t.Fatal("expected Decode to reject an envelope that fails validation, got nil")
	}
}

func TestOperationEnvelope_RoundTrip(t *testing.T) {
	op := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 1, Counter: 1},
		DocumentID:      "doc-1",
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: crdt.ElementID(crdt.RootID),
		ElementID:       crdt.ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}

	env, err := NewOperationEnvelope("doc-1", "client-1", "msg-1", op)
	if err != nil {
		t.Fatalf("NewOperationEnvelope failed: %v", err)
	}

	// Round trip through the wire.
	data, err := Encode(env)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decodedEnv, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	gotOp, err := DecodeOperation(decodedEnv)
	if err != nil {
		t.Fatalf("DecodeOperation failed: %v", err)
	}

	gotInsert, ok := gotOp.(crdt.InsertOperation)
	if !ok {
		t.Fatalf("expected InsertOperation, got %T", gotOp)
	}
	if gotInsert != op {
		t.Errorf("operation round trip mismatch:\n  original = %+v\n  got      = %+v", op, gotInsert)
	}
}

func TestDecodeOperation_WrongEnvelopeType_Errors(t *testing.T) {
	env := Envelope{Type: TypePing, ProtocolVersion: CurrentProtocolVersion}
	_, err := DecodeOperation(env)
	if err == nil {
		t.Fatal("expected an error decoding an operation from a non-operation envelope, got nil")
	}
}

func TestOperationBatchEnvelope_RoundTrip(t *testing.T) {
	root := crdt.ElementID(crdt.RootID)
	a := crdt.InsertOperation{
		OperationID: crdt.OperationID{ClientID: 1, Counter: 1}, DocumentID: "doc-1",
		ClientID: 1, LogicalClock: 1, ParentElementID: root,
		ElementID: crdt.ElementID{ClientID: 1, Counter: 1}, Value: 'a',
	}
	del := crdt.DeleteOperation{
		OperationID: crdt.OperationID{ClientID: 2, Counter: 1}, DocumentID: "doc-1",
		ClientID: 2, LogicalClock: 1, TargetElementID: a.ElementID,
	}
	ops := []crdt.Operation{a, del}

	env, err := NewOperationBatchEnvelope("doc-1", "client-1", "msg-1", ops)
	if err != nil {
		t.Fatalf("NewOperationBatchEnvelope failed: %v", err)
	}

	data, err := Encode(env)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decodedEnv, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	got, err := DecodeOperationBatch(decodedEnv)
	if err != nil {
		t.Fatalf("DecodeOperationBatch failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(got))
	}
	if got[0].(crdt.InsertOperation) != a {
		t.Errorf("first operation mismatch: got %+v, want %+v", got[0], a)
	}
	if got[1].(crdt.DeleteOperation) != del {
		t.Errorf("second operation mismatch: got %+v, want %+v", got[1], del)
	}
}

func TestInitialSyncEnvelope_RoundTrip(t *testing.T) {
	op := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 1, Counter: 1},
		DocumentID:      "doc-1",
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: crdt.ElementID(crdt.RootID),
		ElementID:       crdt.ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}

	env, err := NewInitialSyncEnvelope("doc-1", []crdt.Operation{op}, 42)
	if err != nil {
		t.Fatalf("NewInitialSyncEnvelope failed: %v", err)
	}

	gotOps, gotSeq, err := DecodeInitialSync(env)
	if err != nil {
		t.Fatalf("DecodeInitialSync failed: %v", err)
	}
	if gotSeq != 42 {
		t.Errorf("ServerSequence = %d, want 42", gotSeq)
	}
	if len(gotOps) != 1 || gotOps[0].(crdt.InsertOperation) != op {
		t.Errorf("operations mismatch: got %+v", gotOps)
	}
}

func TestSyncResponseEnvelope_RoundTrip(t *testing.T) {
	op := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 1, Counter: 1},
		DocumentID:      "doc-1",
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: crdt.ElementID(crdt.RootID),
		ElementID:       crdt.ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}

	env, err := NewSyncResponseEnvelope("doc-1", []crdt.Operation{op}, 7, true)
	if err != nil {
		t.Fatalf("NewSyncResponseEnvelope failed: %v", err)
	}

	gotOps, gotSeq, gotHasMore, err := DecodeSyncResponse(env)
	if err != nil {
		t.Fatalf("DecodeSyncResponse failed: %v", err)
	}
	if gotSeq != 7 || !gotHasMore {
		t.Errorf("ServerSequence/HasMore = %d/%v, want 7/true", gotSeq, gotHasMore)
	}
	if len(gotOps) != 1 || gotOps[0].(crdt.InsertOperation) != op {
		t.Errorf("operations mismatch: got %+v", gotOps)
	}
}

func TestEncode_RejectsPreValidationFailures(t *testing.T) {
	// NewOperationEnvelope with an empty messageId should fail
	// ValidateEnvelope internally rather than producing an invalid
	// envelope that would only fail later, at some other layer.
	op := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 1, Counter: 1},
		DocumentID:      "doc-1",
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: crdt.ElementID(crdt.RootID),
		ElementID:       crdt.ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}
	_, err := NewOperationEnvelope("doc-1", "client-1", "", op)
	if err == nil {
		t.Fatal("expected an error constructing an operation envelope with an empty messageId, got nil")
	}
}
