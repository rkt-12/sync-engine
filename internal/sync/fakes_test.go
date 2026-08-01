package sync

import (
	"context"
	"sync"

	"sync-engine/internal/crdt"
	"sync-engine/internal/protocol"
)

// fakePersister is an in-memory stand-in for *store.OperationStore,
// used so DocumentRoom's tests don't need a real Postgres instance.
type fakePersister struct {
	mu        sync.Mutex
	nextSeq   int64
	seqByOpID map[crdt.OperationID]int64
	opsByDoc  map[string][]crdt.Operation // in ascending-sequence order, per document
}

func newFakePersister() *fakePersister {
	return &fakePersister{
		seqByOpID: make(map[crdt.OperationID]int64),
		opsByDoc:  make(map[string][]crdt.Operation),
	}
}

func (p *fakePersister) AppendOperation(ctx context.Context, op crdt.Operation) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if seq, exists := p.seqByOpID[op.ID()]; exists {
		return seq, nil
	}
	p.nextSeq++
	seq := p.nextSeq
	p.seqByOpID[op.ID()] = seq
	p.opsByDoc[op.Document()] = append(p.opsByDoc[op.Document()], op)
	return seq, nil
}

func (p *fakePersister) LoadOperationsAfter(ctx context.Context, documentID string, afterSequence int64, limit int) ([]crdt.Operation, int64, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var (
		page    []crdt.Operation
		highest int64
	)
	hasMore := false
	for _, op := range p.opsByDoc[documentID] {
		seq := p.seqByOpID[op.ID()]
		if seq <= afterSequence {
			continue
		}
		if len(page) == limit {
			hasMore = true
			break
		}
		page = append(page, op)
		highest = seq
	}
	return page, highest, hasMore, nil
}

// fakeRoomClient is an in-memory stand-in for *Client, implementing
// RoomClient without any real socket -- lets DocumentRoom's dispatch,
// broadcast, and slow-consumer logic be tested directly and
// deterministically.
type fakeRoomClient struct {
	id string

	mu           sync.Mutex
	envelopes    []protocol.Envelope
	closed       bool
	closeReason  string
	enqueueFails bool // simulates a slow/full consumer
}

func newFakeRoomClient(id string) *fakeRoomClient {
	return &fakeRoomClient{id: id}
}

func (f *fakeRoomClient) ClientID() string { return f.id }

func (f *fakeRoomClient) Enqueue(env protocol.Envelope) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed || f.enqueueFails {
		return false
	}
	f.envelopes = append(f.envelopes, env)
	return true
}

func (f *fakeRoomClient) Close(reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	f.closeReason = reason
}

func (f *fakeRoomClient) received() []protocol.Envelope {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]protocol.Envelope, len(f.envelopes))
	copy(out, f.envelopes)
	return out
}

func (f *fakeRoomClient) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}
