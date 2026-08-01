package sync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"sync-engine/internal/crdt"
	"sync-engine/internal/protocol"
	"sync-engine/internal/store"
)

// testDSN mirrors internal/store's own test helper -- duplicated here
// (rather than exported from store) since it's a small, test-only
// convenience and this package shouldn't otherwise depend on store's
// internals.
func testDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://postgres:postgres@localhost:5432/sync_engine_test?sslmode=disable"
}

var e2eDocCounter int64

func e2eDocumentID(t *testing.T) string {
	t.Helper()
	n := atomic.AddInt64(&e2eDocCounter, 1)
	return "e2e-doc-" + t.Name() + "-" + time.Now().UTC().Format("20060102T150405.000000000") + "-" + strconv.FormatInt(n, 10)
}

// testServer connects to a real Postgres instance, applies migrations,
// and returns a ready-to-use Server plus the underlying connection pool
// (so tests can also construct DocumentStore/OperationStore directly for
// setup and verification). Skips the test if no database is reachable --
// this is an integration test, a missing database is an environment
// fact.
func testServer(t *testing.T) (*Server, *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := store.Connect(ctx, testDSN())
	if err != nil {
		t.Skipf("skipping: no reachable test database at %s: %v", testDSN(), err)
	}
	t.Cleanup(pool.Close)

	if err := store.Migrate(ctx, pool, os.DirFS("../../migrations")); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	opStore := store.NewOperationStore(pool)
	srv := NewServer(opStore, time.Minute)
	return srv, pool
}

func createDoc(t *testing.T, docs *store.DocumentStore, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := docs.CreateDocument(ctx, id, "E2E Test Document"); err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}
}

// wsClient is a small test helper wrapping a raw *websocket.Conn with
// convenience methods for sending/receiving protocol envelopes -- this
// deliberately does NOT use the real sync.Client, since the point of
// this test is to exercise the server from a genuinely independent,
// minimal client implementation, the way a real (e.g. TypeScript)
// client eventually will.
type wsClient struct {
	t    *testing.T
	conn *websocket.Conn
}

func dialTestServer(t *testing.T, url, documentID, clientID string) *wsClient {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(url, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dialing %s failed: %v", wsURL, err)
	}
	c := &wsClient{t: t, conn: conn}
	t.Cleanup(func() { conn.Close() })

	joinPayload, _ := json.Marshal(protocol.JoinDocumentPayload{})
	c.send(protocol.Envelope{
		Type:            protocol.TypeJoinDocument,
		ProtocolVersion: protocol.CurrentProtocolVersion,
		DocumentID:      documentID,
		ClientID:        clientID,
		Payload:         joinPayload,
	})
	return c
}

func (c *wsClient) send(env protocol.Envelope) {
	c.t.Helper()
	data, err := protocol.Encode(env)
	if err != nil {
		c.t.Fatalf("encoding envelope failed: %v", err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.t.Fatalf("writing message failed: %v", err)
	}
}

func (c *wsClient) receive() protocol.Envelope {
	c.t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, raw, err := c.conn.ReadMessage()
	if err != nil {
		c.t.Fatalf("reading message failed: %v", err)
	}
	env, err := protocol.Decode(raw)
	if err != nil {
		c.t.Fatalf("decoding message failed: %v", err)
	}
	return env
}

// receiveType reads envelopes until one of the given type arrives,
// failing the test if none does within a few messages -- used to skip
// past e.g. an unrelated pong if a heartbeat happens to interleave.
func (c *wsClient) receiveType(want protocol.MessageType) protocol.Envelope {
	c.t.Helper()
	for i := 0; i < 5; i++ {
		env := c.receive()
		if env.Type == want {
			return env
		}
	}
	c.t.Fatalf("did not receive a message of type %q within 5 messages", want)
	return protocol.Envelope{}
}

func TestEndToEnd_TwoClients_ConvergeThroughRealServer(t *testing.T) {
	srv, pool := testServer(t)

	docs := store.NewDocumentStore(pool)
	docID := e2eDocumentID(t)
	createDoc(t, docs, docID)

	httpSrv := httptest.NewServer(http.HandlerFunc(srv.ServeWS))
	defer httpSrv.Close()

	clientA := dialTestServer(t, httpSrv.URL, docID, "client-A")
	initialA := clientA.receiveType(protocol.TypeInitialSync)
	opsA, _, err := protocol.DecodeInitialSync(initialA)
	if err != nil {
		t.Fatalf("DecodeInitialSync (A) failed: %v", err)
	}
	if len(opsA) != 0 {
		t.Fatalf("expected empty initial_sync for a fresh document, got %d ops", len(opsA))
	}

	clientB := dialTestServer(t, httpSrv.URL, docID, "client-B")
	clientB.receiveType(protocol.TypeInitialSync) // also empty; not re-checked

	// Client A inserts 'a' at the start of the document.
	opA := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 1, Counter: 1},
		DocumentID:      docID,
		ClientID:        1,
		LogicalClock:    1,
		ParentElementID: crdt.ElementID(crdt.RootID),
		ElementID:       crdt.ElementID{ClientID: 1, Counter: 1},
		Value:           'a',
	}
	envA, err := protocol.NewOperationEnvelope(docID, "client-A", "msg-A-1", opA)
	if err != nil {
		t.Fatalf("NewOperationEnvelope failed: %v", err)
	}
	clientA.send(envA)

	ackA := clientA.receiveType(protocol.TypeAcknowledgement)
	var ackPayloadA protocol.AcknowledgementPayload
	if err := json.Unmarshal(ackA.Payload, &ackPayloadA); err != nil {
		t.Fatalf("decoding acknowledgement failed: %v", err)
	}
	if ackPayloadA.AcknowledgedMessageID != "msg-A-1" {
		t.Errorf("acknowledgement references messageId %q, want %q", ackPayloadA.AcknowledgedMessageID, "msg-A-1")
	}

	broadcastToB := clientB.receiveType(protocol.TypeOperation)
	gotOpAtB, err := protocol.DecodeOperation(broadcastToB)
	if err != nil {
		t.Fatalf("decoding broadcast operation failed: %v", err)
	}
	if gotOpAtB.(crdt.InsertOperation) != opA {
		t.Errorf("operation received by B mismatch: got %+v, want %+v", gotOpAtB, opA)
	}

	// Client B inserts 'b' after A's element.
	opB := crdt.InsertOperation{
		OperationID:     crdt.OperationID{ClientID: 2, Counter: 1},
		DocumentID:      docID,
		ClientID:        2,
		LogicalClock:    1,
		ParentElementID: opA.ElementID,
		ElementID:       crdt.ElementID{ClientID: 2, Counter: 1},
		Value:           'b',
	}
	envB, err := protocol.NewOperationEnvelope(docID, "client-B", "msg-B-1", opB)
	if err != nil {
		t.Fatalf("NewOperationEnvelope failed: %v", err)
	}
	clientB.send(envB)

	clientB.receiveType(protocol.TypeAcknowledgement)
	broadcastToA := clientA.receiveType(protocol.TypeOperation)
	gotOpAtA, err := protocol.DecodeOperation(broadcastToA)
	if err != nil {
		t.Fatalf("decoding broadcast operation failed: %v", err)
	}
	if gotOpAtA.(crdt.InsertOperation) != opB {
		t.Errorf("operation received by A mismatch: got %+v, want %+v", gotOpAtA, opB)
	}

	// Verify durable persistence: reconstruct the document purely from
	// Postgres, independent of anything held in server memory.
	opStore := store.NewOperationStore(pool)
	replayed, err := opStore.Replay(context.Background(), docID)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if got := replayed.Materialize(); got != "ab" {
		t.Errorf("document replayed from Postgres = %q, want %q", got, "ab")
	}

	// A third client joining now should catch up via initial_sync,
	// seeing both operations without having been connected for either.
	clientC := dialTestServer(t, httpSrv.URL, docID, "client-C")
	initialC := clientC.receiveType(protocol.TypeInitialSync)
	opsC, _, err := protocol.DecodeInitialSync(initialC)
	if err != nil {
		t.Fatalf("DecodeInitialSync (C) failed: %v", err)
	}
	if len(opsC) != 2 {
		t.Fatalf("expected client C's initial_sync to contain 2 operations, got %d", len(opsC))
	}
	docC := crdt.NewDocument(docID)
	if err := docC.ApplyBatch(opsC); err != nil {
		t.Fatalf("applying client C's initial_sync operations failed: %v", err)
	}
	if got := docC.Materialize(); got != "ab" {
		t.Errorf("client C's reconstructed document = %q, want %q", got, "ab")
	}
}
