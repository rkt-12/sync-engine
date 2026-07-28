package protocol

import (
	"encoding/json"
	"fmt"

	"sync-engine/internal/crdt"
)

// Encode marshals env to its canonical wire JSON.
func Encode(env Envelope) ([]byte, error) {
	data, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("protocol: encoding envelope: %w", err)
	}
	return data, nil
}

// Decode unmarshals raw wire JSON into an Envelope and validates it.
func Decode(raw []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Envelope{}, fmt.Errorf("protocol: decoding envelope: invalid JSON: %w", err)
	}
	if err := ValidateEnvelope(env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// NewOperationEnvelope builds a validated TypeOperation envelope
// carrying op, encoded via crdt.MarshalOperation as the Payload.
func NewOperationEnvelope(documentID, clientID, messageID string, op crdt.Operation) (Envelope, error) {
	opJSON, err := crdt.MarshalOperation(op)
	if err != nil {
		return Envelope{}, fmt.Errorf("protocol: encoding operation for envelope: %w", err)
	}
	env := Envelope{
		Type:            TypeOperation,
		ProtocolVersion: CurrentProtocolVersion,
		DocumentID:      documentID,
		ClientID:        clientID,
		MessageID:       messageID,
		Payload:         json.RawMessage(opJSON),
	}
	if err := ValidateEnvelope(env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// DecodeOperation extracts the crdt.Operation carried by a TypeOperation envelope's Payload.
func DecodeOperation(env Envelope) (crdt.Operation, error) {
	if env.Type != TypeOperation {
		return nil, fmt.Errorf("protocol: DecodeOperation: envelope type is %q, want %q", env.Type, TypeOperation)
	}
	op, err := crdt.UnmarshalOperation(env.Payload)
	if err != nil {
		return nil, fmt.Errorf("protocol: decoding operation from envelope: %w", err)
	}
	return op, nil
}

// NewOperationBatchEnvelope builds a validated TypeOperationBatch
// envelope carrying ops, each encoded via crdt.MarshalOperation.
func NewOperationBatchEnvelope(documentID, clientID, messageID string, ops []crdt.Operation) (Envelope, error) {
	encoded := make([]json.RawMessage, 0, len(ops))
	for i, op := range ops {
		opJSON, err := crdt.MarshalOperation(op)
		if err != nil {
			return Envelope{}, fmt.Errorf("protocol: encoding operation %d of %d for batch: %w", i+1, len(ops), err)
		}
		encoded = append(encoded, json.RawMessage(opJSON))
	}
	payload, err := json.Marshal(encoded)
	if err != nil {
		return Envelope{}, fmt.Errorf("protocol: encoding operation batch payload: %w", err)
	}
	env := Envelope{
		Type:            TypeOperationBatch,
		ProtocolVersion: CurrentProtocolVersion,
		DocumentID:      documentID,
		ClientID:        clientID,
		MessageID:       messageID,
		Payload:         payload,
	}
	if err := ValidateEnvelope(env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// DecodeOperationBatch extracts the []crdt.Operation carried by a
// TypeOperationBatch envelope's Payload.
func DecodeOperationBatch(env Envelope) ([]crdt.Operation, error) {
	if env.Type != TypeOperationBatch {
		return nil, fmt.Errorf("protocol: DecodeOperationBatch: envelope type is %q, want %q", env.Type, TypeOperationBatch)
	}
	var rawOps []json.RawMessage
	if err := json.Unmarshal(env.Payload, &rawOps); err != nil {
		return nil, fmt.Errorf("protocol: decoding operation batch payload: %w", err)
	}
	ops := make([]crdt.Operation, 0, len(rawOps))
	for i, raw := range rawOps {
		op, err := crdt.UnmarshalOperation(raw)
		if err != nil {
			return nil, fmt.Errorf("protocol: decoding operation %d of %d in batch: %w", i+1, len(rawOps), err)
		}
		ops = append(ops, op)
	}
	return ops, nil
}

// NewInitialSyncEnvelope builds a validated TypeInitialSync envelope
// carrying ops and the server sequence they were read as of.
func NewInitialSyncEnvelope(documentID string, ops []crdt.Operation, serverSequence int64) (Envelope, error) {
	rawOps, err := encodeOperations(ops)
	if err != nil {
		return Envelope{}, err
	}
	payload, err := json.Marshal(InitialSyncPayload{Operations: rawOps, ServerSequence: serverSequence})
	if err != nil {
		return Envelope{}, fmt.Errorf("protocol: encoding initial_sync payload: %w", err)
	}
	env := Envelope{
		Type:            TypeInitialSync,
		ProtocolVersion: CurrentProtocolVersion,
		DocumentID:      documentID,
		Payload:         payload,
	}
	if err := ValidateEnvelope(env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// DecodeInitialSync extracts the operations and server sequence carried
// by a TypeInitialSync envelope's Payload.
func DecodeInitialSync(env Envelope) ([]crdt.Operation, int64, error) {
	if env.Type != TypeInitialSync {
		return nil, 0, fmt.Errorf("protocol: DecodeInitialSync: envelope type is %q, want %q", env.Type, TypeInitialSync)
	}
	var payload InitialSyncPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return nil, 0, fmt.Errorf("protocol: decoding initial_sync payload: %w", err)
	}
	ops, err := decodeOperations(payload.Operations)
	if err != nil {
		return nil, 0, err
	}
	return ops, payload.ServerSequence, nil
}

// NewSyncResponseEnvelope builds a validated TypeSyncResponse envelope.
func NewSyncResponseEnvelope(documentID string, ops []crdt.Operation, serverSequence int64, hasMore bool) (Envelope, error) {
	rawOps, err := encodeOperations(ops)
	if err != nil {
		return Envelope{}, err
	}
	payload, err := json.Marshal(SyncResponsePayload{
		Operations:     rawOps,
		ServerSequence: serverSequence,
		HasMore:        hasMore,
	})
	if err != nil {
		return Envelope{}, fmt.Errorf("protocol: encoding sync_response payload: %w", err)
	}
	env := Envelope{
		Type:            TypeSyncResponse,
		ProtocolVersion: CurrentProtocolVersion,
		DocumentID:      documentID,
		Payload:         payload,
	}
	if err := ValidateEnvelope(env); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// DecodeSyncResponse extracts the payload of a TypeSyncResponse envelope.
func DecodeSyncResponse(env Envelope) (ops []crdt.Operation, serverSequence int64, hasMore bool, err error) {
	if env.Type != TypeSyncResponse {
		return nil, 0, false, fmt.Errorf("protocol: DecodeSyncResponse: envelope type is %q, want %q", env.Type, TypeSyncResponse)
	}
	var payload SyncResponsePayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return nil, 0, false, fmt.Errorf("protocol: decoding sync_response payload: %w", err)
	}
	decoded, err := decodeOperations(payload.Operations)
	if err != nil {
		return nil, 0, false, err
	}
	return decoded, payload.ServerSequence, payload.HasMore, nil
}

func encodeOperations(ops []crdt.Operation) ([]json.RawMessage, error) {
	raw := make([]json.RawMessage, 0, len(ops))
	for i, op := range ops {
		opJSON, err := crdt.MarshalOperation(op)
		if err != nil {
			return nil, fmt.Errorf("protocol: encoding operation %d of %d: %w", i+1, len(ops), err)
		}
		raw = append(raw, json.RawMessage(opJSON))
	}
	return raw, nil
}

func decodeOperations(raw []json.RawMessage) ([]crdt.Operation, error) {
	ops := make([]crdt.Operation, 0, len(raw))
	for i, r := range raw {
		op, err := crdt.UnmarshalOperation(r)
		if err != nil {
			return nil, fmt.Errorf("protocol: decoding operation %d of %d: %w", i+1, len(raw), err)
		}
		ops = append(ops, op)
	}
	return ops, nil
}
