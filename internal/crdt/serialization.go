package crdt

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// wireOperation is the canonical JSON envelope for a single operation.
type wireOperation struct {
	Kind            OperationKind `json:"kind"`
	OperationID     Identifier    `json:"operationId"`
	DocumentID      string        `json:"documentId"`
	ClientID        uint64        `json:"clientId"`
	LogicalClock    uint64        `json:"logicalClock"`
	ParentElementID *Identifier   `json:"parentElementId,omitempty"`
	ElementID       *Identifier   `json:"elementId,omitempty"`
	Value           *string       `json:"value,omitempty"`
	TargetElementID *Identifier   `json:"targetElementId,omitempty"`
}

// MarshalOperation encodes any Operation into its canonical JSON form.
func MarshalOperation(op Operation) ([]byte, error) {
	switch o := op.(type) {
	case InsertOperation:
		value := string(o.Value)
		parent := Identifier(o.ParentElementID)
		element := Identifier(o.ElementID)
		w := wireOperation{
			Kind:            KindInsert,
			OperationID:     Identifier(o.OperationID),
			DocumentID:      o.DocumentID,
			ClientID:        o.ClientID,
			LogicalClock:    o.LogicalClock,
			ParentElementID: &parent,
			ElementID:       &element,
			Value:           &value,
		}
		return json.Marshal(w)

	case DeleteOperation:
		target := Identifier(o.TargetElementID)
		w := wireOperation{
			Kind:            KindDelete,
			OperationID:     Identifier(o.OperationID),
			DocumentID:      o.DocumentID,
			ClientID:        o.ClientID,
			LogicalClock:    o.LogicalClock,
			TargetElementID: &target,
		}
		return json.Marshal(w)

	default:
		// Unreachable for well-typed callers, but Operation is an
		// interface, a future third implementation would otherwise
		// silently fail to serialize. Fail loudly instead.
		return nil, fmt.Errorf("crdt: MarshalOperation: unsupported operation type %T", op)
	}
}

// UnmarshalOperation decodes a canonical JSON operation produced by MarshalOperation back into
// a concrete InsertOperation or DeleteOperation, returned as the Operation interface.
func UnmarshalOperation(data []byte) (Operation, error) {
	var w wireOperation
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("crdt: UnmarshalOperation: invalid JSON: %w", err)
	}

	switch w.Kind {
	case KindInsert:
		if w.ParentElementID == nil {
			return nil, fmt.Errorf("crdt: UnmarshalOperation: insert operation missing parentElementId")
		}
		if w.ElementID == nil {
			return nil, fmt.Errorf("crdt: UnmarshalOperation: insert operation missing elementId")
		}
		if w.Value == nil {
			return nil, fmt.Errorf("crdt: UnmarshalOperation: insert operation missing value")
		}
		r, size := utf8.DecodeRuneInString(*w.Value)
		if r == utf8.RuneError || size != len(*w.Value) {
			return nil, fmt.Errorf("crdt: UnmarshalOperation: value must be exactly one character, got %q", *w.Value)
		}
		return InsertOperation{
			OperationID:     OperationID(w.OperationID),
			DocumentID:      w.DocumentID,
			ClientID:        w.ClientID,
			LogicalClock:    w.LogicalClock,
			ParentElementID: ElementID(*w.ParentElementID),
			ElementID:       ElementID(*w.ElementID),
			Value:           r,
		}, nil

	case KindDelete:
		if w.TargetElementID == nil {
			return nil, fmt.Errorf("crdt: UnmarshalOperation: delete operation missing targetElementId")
		}
		return DeleteOperation{
			OperationID:     OperationID(w.OperationID),
			DocumentID:      w.DocumentID,
			ClientID:        w.ClientID,
			LogicalClock:    w.LogicalClock,
			TargetElementID: ElementID(*w.TargetElementID),
		}, nil

	default:
		return nil, fmt.Errorf("crdt: UnmarshalOperation: unknown kind %q", w.Kind)
	}
}
