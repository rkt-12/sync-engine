package crdt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarshalUnmarshalInsertOperation_RoundTrip(t *testing.T) {
	original := InsertOperation{
		OperationID:     OperationID{ClientID: 1, Counter: 5},
		DocumentID:      "doc-1",
		ClientID:        1,
		LogicalClock:    5,
		ParentElementID: ElementID(RootID),
		ElementID:       ElementID{ClientID: 1, Counter: 5},
		Value:           'a',
	}

	data, err := MarshalOperation(original)
	if err != nil {
		t.Fatalf("MarshalOperation failed: %v", err)
	}

	decoded, err := UnmarshalOperation(data)
	if err != nil {
		t.Fatalf("UnmarshalOperation failed: %v", err)
	}

	got, ok := decoded.(InsertOperation)
	if !ok {
		t.Fatalf("expected decoded value to be InsertOperation, got %T", decoded)
	}
	if got != original {
		t.Errorf("round trip mismatch:\n  original = %+v\n  got      = %+v", original, got)
	}
}

func TestMarshalUnmarshalDeleteOperation_RoundTrip(t *testing.T) {
	original := DeleteOperation{
		OperationID:     OperationID{ClientID: 2, Counter: 9},
		DocumentID:      "doc-1",
		ClientID:        2,
		LogicalClock:    9,
		TargetElementID: ElementID{ClientID: 1, Counter: 5},
	}

	data, err := MarshalOperation(original)
	if err != nil {
		t.Fatalf("MarshalOperation failed: %v", err)
	}

	decoded, err := UnmarshalOperation(data)
	if err != nil {
		t.Fatalf("UnmarshalOperation failed: %v", err)
	}

	got, ok := decoded.(DeleteOperation)
	if !ok {
		t.Fatalf("expected decoded value to be DeleteOperation, got %T", decoded)
	}
	if got != original {
		t.Errorf("round trip mismatch:\n  original = %+v\n  got      = %+v", original, got)
	}
}

func TestMarshalOperation_InsertOmitsTargetElementID(t *testing.T) {
	op := InsertOperation{
		OperationID:     OperationID{ClientID: 1, Counter: 1},
		DocumentID:      "doc-1",
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: ElementID(RootID),
		ElementID:       ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}

	data, err := MarshalOperation(op)
	if err != nil {
		t.Fatalf("MarshalOperation failed: %v", err)
	}

	if strings.Contains(string(data), "targetElementId") {
		t.Errorf("insert operation JSON should not contain targetElementId, got: %s", data)
	}
}

func TestMarshalOperation_DeleteOmitsInsertOnlyFields(t *testing.T) {
	op := DeleteOperation{
		OperationID:     OperationID{ClientID: 1, Counter: 1},
		DocumentID:      "doc-1",
		ClientID:        1,
		LogicalClock:    1,
		TargetElementID: ElementID{ClientID: 0, Counter: 0},
	}

	data, err := MarshalOperation(op)
	if err != nil {
		t.Fatalf("MarshalOperation failed: %v", err)
	}

	for _, field := range []string{"parentElementId", "elementId", "value"} {
		if strings.Contains(string(data), field) {
			t.Errorf("delete operation JSON should not contain %q, got: %s", field, data)
		}
	}
}

func TestUnmarshalOperation_UnknownKind_Errors(t *testing.T) {
	input := `{"kind":"replace","operationId":{"clientId":1,"counter":1},"documentId":"doc-1","clientId":1,"logicalClock":1}`

	_, err := UnmarshalOperation([]byte(input))
	if err == nil {
		t.Fatal("expected an error for unknown kind, got nil")
	}
}

func TestUnmarshalOperation_InsertMissingElementID_Errors(t *testing.T) {
	input := `{
		"kind": "insert",
		"operationId": {"clientId":1,"counter":1},
		"documentId": "doc-1",
		"clientId": 1,
		"logicalClock": 1,
		"parentElementId": {"clientId":0,"counter":0},
		"value": "a"
	}`

	_, err := UnmarshalOperation([]byte(input))
	if err == nil {
		t.Fatal("expected an error for insert operation missing elementId, got nil")
	}
}

func TestUnmarshalOperation_DeleteMissingTargetElementID_Errors(t *testing.T) {
	input := `{
		"kind": "delete",
		"operationId": {"clientId":1,"counter":1},
		"documentId": "doc-1",
		"clientId": 1,
		"logicalClock": 1
	}`

	_, err := UnmarshalOperation([]byte(input))
	if err == nil {
		t.Fatal("expected an error for delete operation missing targetElementId, got nil")
	}
}

func TestUnmarshalOperation_ValueMustBeSingleCharacter(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty string", "", true},
		{"two ascii characters", "ab", true},
		{"single ascii character", "a", false},
		{"single multibyte character", "é", false},
		{"single emoji (multi-byte rune)", "🙂", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := wireOperation{
				Kind:            KindInsert,
				OperationID:     Identifier{ClientID: 1, Counter: 1},
				DocumentID:      "doc-1",
				ClientID:        1,
				LogicalClock:    1,
				ParentElementID: &Identifier{},
				ElementID:       &Identifier{ClientID: 1, Counter: 1},
				Value:           &tt.value,
			}
			data, err := json.Marshal(wire)
			if err != nil {
				t.Fatalf("failed to marshal test fixture: %v", err)
			}

			_, err = UnmarshalOperation(data)
			if tt.wantErr && err == nil {
				t.Errorf("value %q: expected an error, got nil", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("value %q: expected no error, got %v", tt.value, err)
			}
		})
	}
}

func TestUnmarshalOperation_InvalidJSON_Errors(t *testing.T) {
	_, err := UnmarshalOperation([]byte("not json"))
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestMarshalOperation_FieldOrderIsStable(t *testing.T) {
	op := InsertOperation{
		OperationID:     OperationID{ClientID: 1, Counter: 1},
		DocumentID:      "doc-1",
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: ElementID(RootID),
		ElementID:       ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}

	data, err := MarshalOperation(op)
	if err != nil {
		t.Fatalf("MarshalOperation failed: %v", err)
	}

	want := `{"kind":"insert","operationId":{"clientId":1,"counter":1},"documentId":"doc-1","clientId":1,"logicalClock":1,"parentElementId":{"clientId":0,"counter":0},"elementId":{"clientId":1,"counter":1},"value":"a"}`
	if string(data) != want {
		t.Errorf("field order/shape mismatch:\n  got:  %s\n  want: %s", data, want)
	}
}
