package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"sync-engine/internal/crdt"
	"sync-engine/internal/protocol"
)

// loadPageSize bounds how many rows a single LoadOperationsAfter call
// fetches while reconstructing a document's full history
const loadPageSize = 1000

// RoomClient is the interface DocumentRoom uses to interact with a connected client
type RoomClient interface {
	// ClientID returns this client's protocol-level identifier
	ClientID() string

	// Enqueue attempts to hand env to this client for delivery and never blocks
	Enqueue(env protocol.Envelope) (ok bool)

	// Close disconnects the client with a human-readable reason.
	Close(reason string)
}

// DocumentRoom owns exactly one document's in-memory CRDT replica and
// the set of clients currently connected to it.
type DocumentRoom struct {
	documentID string
	persister  Persister

	mu       sync.Mutex
	document *crdt.Document
	clients  map[string]RoomClient
}

// NewDocumentRoom constructs a room for documentID, reconstructing its
// current CRDT state by replaying every persisted operation
func NewDocumentRoom(ctx context.Context, documentID string, persister Persister) (*DocumentRoom, error) {
	ops, _, err := loadAllOperations(ctx, persister, documentID)
	if err != nil {
		return nil, fmt.Errorf("room: loading operations for document %q: %w", documentID, err)
	}

	doc := crdt.NewDocument(documentID)
	if err := doc.ApplyBatch(ops); err != nil {
		return nil, fmt.Errorf("room: replaying operations for document %q: %w", documentID, err)
	}

	return &DocumentRoom{
		documentID: documentID,
		persister:  persister,
		document:   doc,
		clients:    make(map[string]RoomClient),
	}, nil
}

// loadAllOperations pages through every persisted operation for documentID via Persister.
func loadAllOperations(ctx context.Context, persister Persister, documentID string) ([]crdt.Operation, int64, error) {
	var (
		all   []crdt.Operation
		after int64
	)
	for {
		page, highest, hasMore, err := persister.LoadOperationsAfter(ctx, documentID, after, loadPageSize)
		if err != nil {
			return nil, 0, err
		}
		all = append(all, page...)
		if !hasMore {
			return all, highest, nil
		}
		after = highest
	}
}

// Join registers client in the room.
func (r *DocumentRoom) Join(client RoomClient) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if old, exists := r.clients[client.ClientID()]; exists {
		old.Close("duplicate connection: replaced by a newer connection for the same client")
	}
	r.clients[client.ClientID()] = client
}

// Leave unregisters client, but only if it is still the currently registered client for its ClientID
func (r *DocumentRoom) Leave(client RoomClient) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if current, exists := r.clients[client.ClientID()]; exists && current == client {
		delete(r.clients, client.ClientID())
	}
}

// IsEmpty reports whether the room currently has no connected clients
func (r *DocumentRoom) IsEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients) == 0
}

// SendInitialSync loads the document's full operation history
func (r *DocumentRoom) SendInitialSync(ctx context.Context, to RoomClient) error {
	ops, highest, err := loadAllOperations(ctx, r.persister, r.documentID)
	if err != nil {
		return fmt.Errorf("room: loading operations for initial_sync: %w", err)
	}
	env, err := protocol.NewInitialSyncEnvelope(r.documentID, ops, highest)
	if err != nil {
		return fmt.Errorf("room: building initial_sync envelope: %w", err)
	}
	if !to.Enqueue(env) {
		to.Close("slow consumer: could not deliver initial_sync")
	}
	return nil
}

// HandleOperation persists op, applies it to the room's in-memory
// replica, acknowledges it to the originating client, and broadcasts it
// to every other connected client.
func (r *DocumentRoom) HandleOperation(ctx context.Context, from RoomClient, op crdt.Operation, messageID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	sequenceID, err := r.persister.AppendOperation(ctx, op)
	if err != nil {
		return fmt.Errorf("room: persisting operation: %w", err)
	}

	if err := r.document.Apply(op); err != nil {
		// Persisted but not reflected in this room's in-memory cache.
		return fmt.Errorf("room: applying operation to in-memory replica (persisted as seq=%d): %w", sequenceID, err)
	}

	r.acknowledge(from, messageID, sequenceID)
	r.broadcastOperation(from, messageID, op)
	return nil
}

// HandleOperationBatch is HandleOperation's counterpart for a batch of
// operations arriving together (e.g. offline edits flushed on reconnect)
func (r *DocumentRoom) HandleOperationBatch(ctx context.Context, from RoomClient, ops []crdt.Operation, messageID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastSequence int64
	for i, op := range ops {
		sequenceID, err := r.persister.AppendOperation(ctx, op)
		if err != nil {
			return fmt.Errorf("room: persisting operation %d of %d in batch: %w", i+1, len(ops), err)
		}
		if err := r.document.Apply(op); err != nil {
			return fmt.Errorf("room: applying operation %d of %d in batch (persisted as seq=%d): %w", i+1, len(ops), sequenceID, err)
		}
		lastSequence = sequenceID
	}

	r.acknowledge(from, messageID, lastSequence)

	if len(ops) == 0 {
		return nil
	}
	broadcastEnv, err := protocol.NewOperationBatchEnvelope(r.documentID, from.ClientID(), messageID, ops)
	if err != nil {
		return fmt.Errorf("room: building broadcast batch envelope: %w", err)
	}
	r.broadcastEnvelope(from, broadcastEnv, true)
	return nil
}

// HandleSyncRequest answers a client's catch-up request.
func (r *DocumentRoom) HandleSyncRequest(ctx context.Context, from RoomClient, lastKnownServerSequence int64) error {
	const syncPageSize = 500
	ops, highest, hasMore, err := r.persister.LoadOperationsAfter(ctx, r.documentID, lastKnownServerSequence, syncPageSize)
	if err != nil {
		return fmt.Errorf("room: loading operations for sync_request: %w", err)
	}
	env, err := protocol.NewSyncResponseEnvelope(r.documentID, ops, highest, hasMore)
	if err != nil {
		return fmt.Errorf("room: building sync_response envelope: %w", err)
	}
	if !from.Enqueue(env) {
		from.Close("slow consumer: could not deliver sync_response")
	}
	return nil
}

// BroadcastEphemeral relays a presence_update or cursor_update envelope
// to every other client in the room.
func (r *DocumentRoom) BroadcastEphemeral(from RoomClient, env protocol.Envelope) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broadcastEnvelope(from, env, false)
}

// acknowledge sends an acknowledgement envelope to the originating client, closing it if delivery fails
func (r *DocumentRoom) acknowledge(from RoomClient, messageID string, sequenceID int64) {
	payload, err := json.Marshal(protocol.AcknowledgementPayload{
		AcknowledgedMessageID: messageID,
		ServerSequence:        sequenceID,
	})
	if err != nil {
		return
	}
	env := protocol.Envelope{
		Type:            protocol.TypeAcknowledgement,
		ProtocolVersion: protocol.CurrentProtocolVersion,
		MessageID:       messageID,
		Payload:         payload,
	}
	if !from.Enqueue(env) {
		from.Close("slow consumer: could not deliver acknowledgement")
	}
}

// broadcastOperation builds and sends a single-operation broadcast envelope to every client except the originator.
func (r *DocumentRoom) broadcastOperation(from RoomClient, messageID string, op crdt.Operation) {
	env, err := protocol.NewOperationEnvelope(r.documentID, from.ClientID(), messageID, op)
	if err != nil {
		return
	}
	r.broadcastEnvelope(from, env, true)
}

// broadcastEnvelope sends env to every client except from.
func (r *DocumentRoom) broadcastEnvelope(from RoomClient, env protocol.Envelope, closeOnFailure bool) {
	for id, client := range r.clients {
		if id == from.ClientID() {
			continue
		}
		if !client.Enqueue(env) && closeOnFailure {
			client.Close("slow consumer: could not deliver broadcast")
		}
	}
}
