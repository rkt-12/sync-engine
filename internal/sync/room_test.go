package sync

import (
	"context"
	"testing"

	"sync-engine/internal/crdt"
	"sync-engine/internal/protocol"
)

func newTestRoom(t *testing.T, persister Persister, documentID string) *DocumentRoom {
	t.Helper()
	room, err := NewDocumentRoom(context.Background(), documentID, persister)
	if err != nil {
		t.Fatalf("NewDocumentRoom failed: %v", err)
	}
	return room
}

func insertOp(clientID, counter uint64, parent crdt.ElementID, documentID string, value rune) crdt.InsertOperation {
	id := crdt.Identifier{ClientID: clientID, Counter: counter}
	return crdt.InsertOperation{
		OperationID:     crdt.OperationID(id),
		DocumentID:      documentID,
		ClientID:        clientID,
		LogicalClock:    counter,
		ParentElementID: parent,
		ElementID:       crdt.ElementID(id),
		Value:           value,
	}
}

func TestDocumentRoom_JoinAndLeave(t *testing.T) {
	room := newTestRoom(t, newFakePersister(), "doc-1")
	c := newFakeRoomClient("client-1")

	room.Join(c)
	if room.IsEmpty() {
		t.Fatal("expected room not to be empty after Join")
	}

	room.Leave(c)
	if !room.IsEmpty() {
		t.Fatal("expected room to be empty after Leave")
	}
}

func TestDocumentRoom_Join_DuplicateConnection_ClosesOld(t *testing.T) {
	room := newTestRoom(t, newFakePersister(), "doc-1")
	oldClient := newFakeRoomClient("client-1")
	newClient := newFakeRoomClient("client-1") // same ClientID, different connection

	room.Join(oldClient)
	room.Join(newClient)

	if !oldClient.isClosed() {
		t.Error("expected the old connection to be closed when a duplicate joins")
	}
	if newClient.isClosed() {
		t.Error("the new connection should not be closed")
	}
}

func TestDocumentRoom_Leave_DoesNotRemoveReplacementClient(t *testing.T) {
	// Regression test for the exact race described in room.go's Leave
	// doc comment: if the OLD connection's cleanup calls Leave after a
	// NEW connection has already taken its place, the new one must
	// survive.
	room := newTestRoom(t, newFakePersister(), "doc-1")
	oldClient := newFakeRoomClient("client-1")
	newClient := newFakeRoomClient("client-1")

	room.Join(oldClient)
	room.Join(newClient) // replaces oldClient; oldClient.Close() called internally

	room.Leave(oldClient) // oldClient's belated cleanup

	if room.IsEmpty() {
		t.Fatal("expected the replacement client to still be registered after the old one's Leave")
	}
}

func TestDocumentRoom_HandleOperation_AcksAndBroadcasts(t *testing.T) {
	persister := newFakePersister()
	room := newTestRoom(t, persister, "doc-1")

	sender := newFakeRoomClient("client-1")
	receiver := newFakeRoomClient("client-2")
	room.Join(sender)
	room.Join(receiver)

	op := insertOp(1, 1, crdt.ElementID(crdt.RootID), "doc-1", 'a')
	if err := room.HandleOperation(context.Background(), sender, op, "msg-1"); err != nil {
		t.Fatalf("HandleOperation failed: %v", err)
	}

	senderMsgs := sender.received()
	if len(senderMsgs) != 1 || senderMsgs[0].Type != protocol.TypeAcknowledgement {
		t.Fatalf("expected sender to receive exactly 1 acknowledgement, got %+v", senderMsgs)
	}

	receiverMsgs := receiver.received()
	if len(receiverMsgs) != 1 || receiverMsgs[0].Type != protocol.TypeOperation {
		t.Fatalf("expected receiver to receive exactly 1 operation broadcast, got %+v", receiverMsgs)
	}

	gotOp, err := protocol.DecodeOperation(receiverMsgs[0])
	if err != nil {
		t.Fatalf("decoding broadcast operation failed: %v", err)
	}
	if gotOp.(crdt.InsertOperation) != op {
		t.Errorf("broadcast operation mismatch: got %+v, want %+v", gotOp, op)
	}
}

func TestDocumentRoom_HandleOperation_DoesNotBroadcastToSender(t *testing.T) {
	persister := newFakePersister()
	room := newTestRoom(t, persister, "doc-1")

	sender := newFakeRoomClient("client-1")
	room.Join(sender)

	op := insertOp(1, 1, crdt.ElementID(crdt.RootID), "doc-1", 'a')
	if err := room.HandleOperation(context.Background(), sender, op, "msg-1"); err != nil {
		t.Fatalf("HandleOperation failed: %v", err)
	}

	msgs := sender.received()
	for _, m := range msgs {
		if m.Type == protocol.TypeOperation {
			t.Errorf("sender should not receive its own operation as a broadcast, got %+v", m)
		}
	}
}

func TestDocumentRoom_HandleOperation_IsAppliedToInMemoryReplica(t *testing.T) {
	persister := newFakePersister()
	room := newTestRoom(t, persister, "doc-1")
	client := newFakeRoomClient("client-1")
	room.Join(client)

	op := insertOp(1, 1, crdt.ElementID(crdt.RootID), "doc-1", 'a')
	if err := room.HandleOperation(context.Background(), client, op, "msg-1"); err != nil {
		t.Fatalf("HandleOperation failed: %v", err)
	}

	if got := room.document.Materialize(); got != "a" {
		t.Errorf("room's in-memory document = %q, want %q", got, "a")
	}
}

func TestDocumentRoom_HandleOperation_DuplicateMessage_Idempotent(t *testing.T) {
	persister := newFakePersister()
	room := newTestRoom(t, persister, "doc-1")
	sender := newFakeRoomClient("client-1")
	receiver := newFakeRoomClient("client-2")
	room.Join(sender)
	room.Join(receiver)

	op := insertOp(1, 1, crdt.ElementID(crdt.RootID), "doc-1", 'a')

	if err := room.HandleOperation(context.Background(), sender, op, "msg-1"); err != nil {
		t.Fatalf("first HandleOperation failed: %v", err)
	}
	if err := room.HandleOperation(context.Background(), sender, op, "msg-1-retry"); err != nil {
		t.Fatalf("second (duplicate) HandleOperation failed: %v", err)
	}

	if got := room.document.Materialize(); got != "a" {
		t.Errorf("document = %q after duplicate delivery, want %q (no double character)", got, "a")
	}

	// The receiver should still only see the operation reflected once in
	// its materialized content, even though it received two broadcast
	// envelopes -- applying the second is a harmless no-op on their side.
	opCount := 0
	for _, m := range receiver.received() {
		if m.Type == protocol.TypeOperation {
			opCount++
		}
	}
	if opCount != 2 {
		t.Errorf("expected 2 broadcast envelopes (both retries relayed), got %d", opCount)
	}
}

func TestDocumentRoom_HandleOperation_SlowConsumer_ClosesReceiver(t *testing.T) {
	persister := newFakePersister()
	room := newTestRoom(t, persister, "doc-1")

	sender := newFakeRoomClient("client-1")
	slowReceiver := newFakeRoomClient("client-2")
	slowReceiver.enqueueFails = true
	room.Join(sender)
	room.Join(slowReceiver)

	op := insertOp(1, 1, crdt.ElementID(crdt.RootID), "doc-1", 'a')
	if err := room.HandleOperation(context.Background(), sender, op, "msg-1"); err != nil {
		t.Fatalf("HandleOperation failed: %v", err)
	}

	if !slowReceiver.isClosed() {
		t.Error("expected the slow-consuming receiver to be closed after a failed broadcast delivery")
	}
	// The sender's own acknowledgement delivery succeeded, so it should
	// not have been closed.
	if sender.isClosed() {
		t.Error("sender should not be closed")
	}
}

func TestDocumentRoom_HandleOperationBatch(t *testing.T) {
	persister := newFakePersister()
	room := newTestRoom(t, persister, "doc-1")
	sender := newFakeRoomClient("client-1")
	receiver := newFakeRoomClient("client-2")
	room.Join(sender)
	room.Join(receiver)

	a := insertOp(1, 1, crdt.ElementID(crdt.RootID), "doc-1", 'a')
	b := insertOp(1, 2, a.ElementID, "doc-1", 'b')
	ops := []crdt.Operation{a, b}

	if err := room.HandleOperationBatch(context.Background(), sender, ops, "batch-1"); err != nil {
		t.Fatalf("HandleOperationBatch failed: %v", err)
	}

	if got := room.document.Materialize(); got != "ab" {
		t.Errorf("document = %q, want %q", got, "ab")
	}

	receiverMsgs := receiver.received()
	if len(receiverMsgs) != 1 || receiverMsgs[0].Type != protocol.TypeOperationBatch {
		t.Fatalf("expected receiver to get exactly 1 operation_batch envelope, got %+v", receiverMsgs)
	}
	gotOps, err := protocol.DecodeOperationBatch(receiverMsgs[0])
	if err != nil {
		t.Fatalf("decoding batch failed: %v", err)
	}
	if len(gotOps) != 2 {
		t.Fatalf("expected 2 operations in the batch, got %d", len(gotOps))
	}
}

func TestDocumentRoom_HandleSyncRequest_ReturnsOperationsAfterOffset(t *testing.T) {
	persister := newFakePersister()
	room := newTestRoom(t, persister, "doc-1")
	client := newFakeRoomClient("client-1")
	room.Join(client)

	a := insertOp(1, 1, crdt.ElementID(crdt.RootID), "doc-1", 'a')
	b := insertOp(1, 2, a.ElementID, "doc-1", 'b')
	seqA, err := persister.AppendOperation(context.Background(), a)
	if err != nil {
		t.Fatalf("AppendOperation(a) failed: %v", err)
	}
	if _, err := persister.AppendOperation(context.Background(), b); err != nil {
		t.Fatalf("AppendOperation(b) failed: %v", err)
	}

	if err := room.HandleSyncRequest(context.Background(), client, seqA); err != nil {
		t.Fatalf("HandleSyncRequest failed: %v", err)
	}

	msgs := client.received()
	if len(msgs) != 1 || msgs[0].Type != protocol.TypeSyncResponse {
		t.Fatalf("expected exactly 1 sync_response, got %+v", msgs)
	}
	gotOps, seq, hasMore, err := protocol.DecodeSyncResponse(msgs[0])
	if err != nil {
		t.Fatalf("DecodeSyncResponse failed: %v", err)
	}
	if hasMore {
		t.Error("expected hasMore=false")
	}
	if len(gotOps) != 1 || gotOps[0].(crdt.InsertOperation) != b {
		t.Errorf("expected only operation b after seqA, got %+v", gotOps)
	}
	_ = seq
}

func TestDocumentRoom_BroadcastEphemeral_DoesNotCloseSlowConsumer(t *testing.T) {
	room := newTestRoom(t, newFakePersister(), "doc-1")
	sender := newFakeRoomClient("client-1")
	slowReceiver := newFakeRoomClient("client-2")
	slowReceiver.enqueueFails = true
	room.Join(sender)
	room.Join(slowReceiver)

	env := protocol.Envelope{
		Type:            protocol.TypeCursorUpdate,
		ProtocolVersion: protocol.CurrentProtocolVersion,
		DocumentID:      "doc-1",
		ClientID:        "client-1",
		Payload:         []byte(`{"afterElementIdClient":0,"afterElementIdCounter":0}`),
	}
	room.BroadcastEphemeral(sender, env)

	if slowReceiver.isClosed() {
		t.Error("a slow consumer should NOT be closed for a dropped ephemeral (presence/cursor) message")
	}
}

func TestDocumentRoom_SendInitialSync(t *testing.T) {
	persister := newFakePersister()
	ctx := context.Background()

	a := insertOp(1, 1, crdt.ElementID(crdt.RootID), "doc-1", 'a')
	if _, err := persister.AppendOperation(ctx, a); err != nil {
		t.Fatalf("AppendOperation failed: %v", err)
	}

	room := newTestRoom(t, persister, "doc-1")
	client := newFakeRoomClient("client-1")

	if err := room.SendInitialSync(ctx, client); err != nil {
		t.Fatalf("SendInitialSync failed: %v", err)
	}

	msgs := client.received()
	if len(msgs) != 1 || msgs[0].Type != protocol.TypeInitialSync {
		t.Fatalf("expected exactly 1 initial_sync envelope, got %+v", msgs)
	}
	gotOps, _, err := protocol.DecodeInitialSync(msgs[0])
	if err != nil {
		t.Fatalf("DecodeInitialSync failed: %v", err)
	}
	if len(gotOps) != 1 || gotOps[0].(crdt.InsertOperation) != a {
		t.Errorf("expected initial_sync to contain operation a, got %+v", gotOps)
	}
}

func TestNewDocumentRoom_ReplaysExistingOperations(t *testing.T) {
	persister := newFakePersister()
	ctx := context.Background()

	a := insertOp(1, 1, crdt.ElementID(crdt.RootID), "doc-1", 'a')
	b := insertOp(1, 2, a.ElementID, "doc-1", 'b')
	if _, err := persister.AppendOperation(ctx, a); err != nil {
		t.Fatalf("AppendOperation(a) failed: %v", err)
	}
	if _, err := persister.AppendOperation(ctx, b); err != nil {
		t.Fatalf("AppendOperation(b) failed: %v", err)
	}

	room, err := NewDocumentRoom(ctx, "doc-1", persister)
	if err != nil {
		t.Fatalf("NewDocumentRoom failed: %v", err)
	}
	if got := room.document.Materialize(); got != "ab" {
		t.Errorf("newly constructed room's document = %q, want %q (should replay existing history)", got, "ab")
	}
}

func TestDocumentRoom_ConcurrentOperations_NoRace(t *testing.T) {
	persister := newFakePersister()
	room := newTestRoom(t, persister, "doc-1")
	client := newFakeRoomClient("client-1")
	room.Join(client)

	const n = 50
	errs := make(chan error, n)
	root := crdt.ElementID(crdt.RootID)
	for i := 0; i < n; i++ {
		go func(i int) {
			op := insertOp(uint64(i+1), 1, root, "doc-1", 'x')
			errs <- room.HandleOperation(context.Background(), client, op, "msg")
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent HandleOperation failed: %v", err)
		}
	}

	if got := len([]rune(room.document.Materialize())); got != n {
		t.Errorf("expected %d characters after %d concurrent inserts, got %d", n, n, got)
	}
}
