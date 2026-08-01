# WebSocket Protocol

Status: Envelope, message types, payloads, and
validation live in `internal/protocol`; connection management, rooms,
and the read/write pumps live in `internal/sync`.

## Envelope

Every message, in both directions, is this single wire structure:

```json
{
    "type": "operation",
    "protocolVersion": 1,
    "documentId": "...",
    "clientId": "...",
    "messageId": "...",
    "payload": { }
}
```

`protocolVersion` must equal `1` (`protocol.CurrentProtocolVersion`).
A mismatched version is rejected outright by `ValidateEnvelope` --
there is no negotiation or downgrade path in v1.

`messageId` is the **message-layer** deduplication key -- distinct
from the CRDT-layer `operation_id` deduplication described in
`docs/crdt-specification.md`. A retried WebSocket send (same
`messageId`) is a different concern from an operation being
re-delivered (same `operation_id`); see Invariant 1 for why the CRDT
layer's idempotency doesn't depend on the message layer catching
duplicates at all -- it's defense in depth, not the primary guarantee.

## Message Types

Required envelope fields beyond `type`/`protocolVersion` are enforced
by `protocol.ValidateEnvelope`; see `internal/protocol/validation.go`
for the authoritative list. Summarized:

| Type | Direction | Required fields | Payload |
|---|---|---|---|
| `join_document` | client → server | `documentId`, `clientId` | `JoinDocumentPayload` (empty) |
| `initial_sync` | server → client | `documentId`, `payload` | `InitialSyncPayload` |
| `operation` | both | `documentId`, `clientId`, `messageId`, `payload` | canonical `crdt.Operation` JSON |
| `operation_batch` | client → server | `documentId`, `clientId`, `messageId`, `payload` | array of canonical `crdt.Operation` JSON |
| `acknowledgement` | server → client | `messageId`, `payload` | `AcknowledgementPayload` |
| `sync_request` | client → server | `documentId`, `clientId`, `payload` | `SyncRequestPayload` |
| `sync_response` | server → client | `documentId`, `payload` | `SyncResponsePayload` |
| `presence_update` | both | `documentId`, `clientId`, `payload` | `PresenceUpdatePayload` |
| `cursor_update` | both | `documentId`, `clientId`, `payload` | `CursorUpdatePayload` |
| `error` | server → client | `payload` | `ErrorPayload` |
| `ping` / `pong` | both | (none) | (none) |

### `join_document`

Sent once, right after the WebSocket connection is established.
Requests to join the room for `documentId`. The server responds with
`initial_sync`. No retry/idempotency concerns beyond the connection
itself -- if the connection drops before a response arrives, the
client reconnects and re-sends `join_document`; joining twice is
harmless (Phase 5b: `DocumentRoom` membership is a set).

### `initial_sync`

Sent exactly once per successful join. Carries every operation needed
to materialize current document state (full replay in v1; snapshot +
remainder starting Phase 8) plus `serverSequence`, the offset the
client should remember and send back in a future `sync_request`.

### `operation` / `operation_batch`

Client → server: one (or a batch of) locally-generated operation(s),
already applied to the client's own replica (local-first editing).
Server → client: broadcast of an accepted operation to other room
members.

**Idempotency**: applying the same operation twice via `Document.Apply`
is a no-op (Invariant 1) -- so even if message-layer dedup
(`messageId`) fails to catch a retry, correctness holds. **Retry
behavior**: a client that sent an `operation` and didn't receive an
`acknowledgement` within a timeout should resend the *same* envelope
(same `messageId`, same operation) rather than fabricate a new one.

### `acknowledgement`

Server → client only. Confirms `acknowledgedMessageId` was durably
persisted (`operations` table write succeeded), carrying the assigned
`serverSequence`. The client uses this to advance its own
`lastKnownServerSequence` bookkeeping (`docs/synchronization-protocol.md`)
and to remove the corresponding entry from its local pending-operations
queue once Phase 7 exists.

### `sync_request` / `sync_response`

The reconnection catch-up mechanism specified in
`docs/synchronization-protocol.md`. `sync_response.hasMore` signals
pagination: the client should issue another `sync_request` using the
new `serverSequence` if `hasMore` is true.

### `presence_update` / `cursor_update`

Never touch the durable operation log (Invariant 9). Lossy by design --
the server may drop or coalesce these under load without violating any
correctness property. `CursorUpdatePayload` positions a cursor relative
to a CRDT element identifier, never a numeric index, for the same
reason operations themselves never use one.

### `error`

Sent by the server when a received envelope fails validation, targets
an unknown document, or otherwise cannot be processed. Carries a
machine-readable `code` and a human-readable `message`. Receiving an
`error` does not close the connection by itself (Phase 5b defines
which specific error conditions do warrant closing).

### `ping` / `pong`

Bidirectional heartbeat, no payload. Interval and timeout values are a
Phase 5b connection-manager concern, not a wire-format concern.

## Validation

`protocol.ValidateEnvelope` is the single enforcement point for "no
arbitrary JSON" (project brief, Section 5): protocol version, known
type, and required-fields-per-type are all checked before an envelope
is considered well-formed. `protocol.Decode` calls it automatically;
nothing downstream should need to re-check these basics.

## Size Limits

`protocol.MaxMessageSize` = 64 KiB, defined in the protocol package
even though it can only be *enforced* by whoever reads raw bytes off
the wire (Phase 5b's read pump). This is a deliberate split: the limit
is a protocol-level fact ("what counts as a legal message"), while
enforcement is a connection-management concern.

## Connection Management (`internal/sync`)

- `Client` wraps one `*websocket.Conn`. `WritePump` is the sole
  goroutine that ever writes data frames for a connection (Invariant 7)
  -- everything else sends via a bounded channel (`Enqueue`, capacity
  256), which never blocks: a full channel means a slow consumer, and
  the caller decides whether that warrants disconnecting (durable
  content like operations: yes; ephemeral content like cursor updates:
  no, per Invariant 9).
- Read/write deadlines: `pongWait` = 60s, `pingInterval` = 54s (9/10 of
  pongWait, so a healthy connection always gets a ping well before its
  read deadline could expire).
- `MaxMessageSize` (64 KiB) is enforced via `conn.SetReadLimit` in
  `ReadPump`.
- Duplicate connections from the same `clientId`: **last-connection-wins**
  -- `DocumentRoom.Join` closes the previous connection and replaces it.
- `DocumentRoom` owns one document's in-memory CRDT replica and
  connected-client set, serialized by a plain mutex (not a channel-based
  event loop -- the critical sections are short and CPU-bound, so a
  mutex is simpler and equally correct; see the design note on
  `HandleOperation`).
- `RoomManager` creates rooms lazily and tears them down a configurable
  grace period after the last client leaves, to absorb quick
  reconnects.
- `ConnectionManager` tracks every live connection server-wide, for
  graceful shutdown (`Shutdown` closes everything).
- `join_document` is handled once, before a connection's message loop
  starts (see `Server.ServeWS`) -- not inside the general dispatcher,
  since which document a connection belongs to doesn't change for that
  connection's lifetime in this protocol version.