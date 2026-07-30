package store

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sync-engine/internal/crdt"
)

// OperationStore persists the operation log.
type OperationStore struct {
	pool *pgxpool.Pool
}

// NewOperationStore constructs an OperationStore backed by pool.
func NewOperationStore(pool *pgxpool.Pool) *OperationStore {
	return &OperationStore{pool: pool}
}

const (
	opTypeInsert int16 = 1
	opTypeDelete int16 = 2
)

// AppendOperation persists a single operation.
func (s *OperationStore) AppendOperation(ctx context.Context, op crdt.Operation) (int64, error) {
	switch o := op.(type) {
	case crdt.InsertOperation:
		var sequenceID int64
		err := s.pool.QueryRow(ctx, `
			INSERT INTO operations
				(operation_id, document_id, client_id, logical_clock,
				 operation_type, parent_element_id, value)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (document_id, operation_id)
				DO UPDATE SET operation_id = EXCLUDED.operation_id
			RETURNING sequence_id
		`,
			encodeIdentifier(crdt.Identifier(o.OperationID)),
			o.DocumentID,
			int64(o.ClientID),
			int64(o.LogicalClock),
			opTypeInsert,
			encodeIdentifier(crdt.Identifier(o.ParentElementID)),
			string(o.Value),
		).Scan(&sequenceID)
		if err != nil {
			return 0, fmt.Errorf("store: appending insert operation (doc=%s): %w", o.DocumentID, err)
		}
		return sequenceID, nil

	case crdt.DeleteOperation:
		var sequenceID int64
		err := s.pool.QueryRow(ctx, `
			INSERT INTO operations
				(operation_id, document_id, client_id, logical_clock,
				 operation_type, target_element_id)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (document_id, operation_id)
				DO UPDATE SET operation_id = EXCLUDED.operation_id
			RETURNING sequence_id
		`,
			encodeIdentifier(crdt.Identifier(o.OperationID)),
			o.DocumentID,
			int64(o.ClientID),
			int64(o.LogicalClock),
			opTypeDelete,
			encodeIdentifier(crdt.Identifier(o.TargetElementID)),
		).Scan(&sequenceID)
		if err != nil {
			return 0, fmt.Errorf("store: appending delete operation (doc=%s): %w", o.DocumentID, err)
		}
		return sequenceID, nil

	default:
		return 0, fmt.Errorf("store: unsupported operation type %T", op)
	}
}

// LoadOperations returns every operation persisted for documentID, in server persistence order (sequence_id ascending).
func (s *OperationStore) LoadOperations(ctx context.Context, documentID string) ([]crdt.Operation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT operation_id, client_id, logical_clock, operation_type,
		       parent_element_id, target_element_id, value
		FROM operations
		WHERE document_id = $1
		ORDER BY sequence_id ASC
	`, documentID)
	if err != nil {
		return nil, fmt.Errorf("store: loading operations for document %q: %w", documentID, err)
	}
	defer rows.Close()

	var ops []crdt.Operation
	for rows.Next() {
		op, err := scanOperation(rows, documentID)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: reading operations for document %q: %w", documentID, err)
	}
	return ops, nil
}

func (s *OperationStore) LoadOperationsAfter(ctx context.Context, documentID string, afterSequence int64, limit int) (ops []crdt.Operation, highestSequence int64, hasMore bool, err error) {
	// Fetch one extra row beyond limit purely to detect whether more
	// data exists, without a second round trip (COUNT query) just to
	// answer hasMore.
	rows, err := s.pool.Query(ctx, `
		SELECT sequence_id, operation_id, client_id, logical_clock, operation_type,
		       parent_element_id, target_element_id, value
		FROM operations
		WHERE document_id = $1 AND sequence_id > $2
		ORDER BY sequence_id ASC
		LIMIT $3
	`, documentID, afterSequence, limit+1)
	if err != nil {
		return nil, 0, false, fmt.Errorf("store: loading operations after %d for document %q: %w", afterSequence, documentID, err)
	}
	defer rows.Close()

	type row struct {
		sequenceID int64
		op         crdt.Operation
	}
	var all []row
	for rows.Next() {
		var sequenceID int64
		op, seqErr := scanOperationWithSequence(rows, documentID, &sequenceID)
		if seqErr != nil {
			return nil, 0, false, seqErr
		}
		all = append(all, row{sequenceID: sequenceID, op: op})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, fmt.Errorf("store: reading operations after %d for document %q: %w", afterSequence, documentID, err)
	}

	hasMore = len(all) > limit
	if hasMore {
		all = all[:limit]
	}

	ops = make([]crdt.Operation, 0, len(all))
	for _, r := range all {
		ops = append(ops, r.op)
		highestSequence = r.sequenceID
	}
	return ops, highestSequence, hasMore, nil
}

// Replay loads every persisted operation for documentID and applies them, in the order read back, to a freshly constructed crdt.
func (s *OperationStore) Replay(ctx context.Context, documentID string) (*crdt.Document, error) {
	ops, err := s.LoadOperations(ctx, documentID)
	if err != nil {
		return nil, err
	}
	doc := crdt.NewDocument(documentID)
	if err := doc.ApplyBatch(ops); err != nil {
		return nil, fmt.Errorf("store: replaying operations for document %q: %w", documentID, err)
	}
	return doc, nil
}

// scanOperation decodes one row (as produced by the SELECT in LoadOperations) into a concrete crdt.Operation.
func scanOperation(rows pgx.Rows, documentID string) (crdt.Operation, error) {
	var (
		operationIDBytes []byte
		clientID         int64
		logicalClock     int64
		operationType    int16
		parentBytes      []byte
		targetBytes      []byte
		value            *string
	)
	if err := rows.Scan(&operationIDBytes, &clientID, &logicalClock, &operationType, &parentBytes, &targetBytes, &value); err != nil {
		return nil, fmt.Errorf("store: scanning operation row: %w", err)
	}
	return decodeOperationRow(documentID, operationIDBytes, clientID, logicalClock, operationType, parentBytes, targetBytes, value)
}

func scanOperationWithSequence(rows pgx.Rows, documentID string, sequenceID *int64) (crdt.Operation, error) {
	var (
		operationIDBytes []byte
		clientID         int64
		logicalClock     int64
		operationType    int16
		parentBytes      []byte
		targetBytes      []byte
		value            *string
	)
	if err := rows.Scan(sequenceID, &operationIDBytes, &clientID, &logicalClock, &operationType, &parentBytes, &targetBytes, &value); err != nil {
		return nil, fmt.Errorf("store: scanning operation row with sequence: %w", err)
	}
	return decodeOperationRow(documentID, operationIDBytes, clientID, logicalClock, operationType, parentBytes, targetBytes, value)
}

func decodeOperationRow(documentID string, operationIDBytes []byte, clientID, logicalClock int64, operationType int16, parentBytes, targetBytes []byte, value *string) (crdt.Operation, error) {
	opID, err := decodeIdentifier(operationIDBytes)
	if err != nil {
		return nil, fmt.Errorf("store: decoding operation_id: %w", err)
	}

	switch operationType {
	case opTypeInsert:
		if parentBytes == nil {
			return nil, fmt.Errorf("store: insert operation %+v missing parent_element_id", opID)
		}
		parentID, err := decodeIdentifier(parentBytes)
		if err != nil {
			return nil, fmt.Errorf("store: decoding parent_element_id for operation %+v: %w", opID, err)
		}
		if value == nil {
			return nil, fmt.Errorf("store: insert operation %+v missing value", opID)
		}
		r, size := utf8.DecodeRuneInString(*value)
		if r == utf8.RuneError || size != len(*value) {
			return nil, fmt.Errorf("store: insert operation %+v has invalid value %q", opID, *value)
		}
		return crdt.InsertOperation{
			OperationID:     crdt.OperationID(opID),
			DocumentID:      documentID,
			ClientID:        uint64(clientID),
			LogicalClock:    uint64(logicalClock),
			ParentElementID: crdt.ElementID(parentID),
			// Convention from docs/crdt-specification.md: for an insert,
			// ElementID is numerically identical to OperationID -- one
			// causal event, one identity -- so it is not stored as a
			// separate column.
			ElementID: crdt.ElementID(opID),
			Value:     r,
		}, nil

	case opTypeDelete:
		if targetBytes == nil {
			return nil, fmt.Errorf("store: delete operation %+v missing target_element_id", opID)
		}
		targetID, err := decodeIdentifier(targetBytes)
		if err != nil {
			return nil, fmt.Errorf("store: decoding target_element_id for operation %+v: %w", opID, err)
		}
		return crdt.DeleteOperation{
			OperationID:     crdt.OperationID(opID),
			DocumentID:      documentID,
			ClientID:        uint64(clientID),
			LogicalClock:    uint64(logicalClock),
			TargetElementID: crdt.ElementID(targetID),
		}, nil

	default:
		return nil, fmt.Errorf("store: unknown operation_type %d for operation %+v", operationType, opID)
	}
}
