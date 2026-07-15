# WebSocket Protocol

Status: STUB — full specification is a Phase 5 deliverable. This file
records the decisions already made in Phase 0 so Phase 5 starts from
agreed ground rather than a blank page.

## Decided in Phase 0

- Every message is a versioned envelope:

```json
{
    "type": "operation",
    "protocolVersion": 1,
    "documentId": "...",
    "clientId": "...",
    "messageId": "...",
    "payload": {}
}
```

- `messageId` is used for message-layer deduplication (distinct from
  CRDT-layer `operation_id` deduplication — see
  `docs/crdt-specification.md`, Deduplication section, and
  `docs/invariants.md` Invariant 1).
- Message types required: `join_document`, `initial_sync`,
  `operation`, `operation_batch`, `acknowledgement`, `sync_request`,
  `sync_response`, `presence_update`, `cursor_update`, `error`,
  `ping`, `pong`.
- Reconnection/catch-up uses the server's persisted-operation sequence
  offset (`sync_request{lastKnownServerSeq}` /
  `sync_response`), not a version vector — see
  `docs/synchronization-protocol.md` for the reasoning.
- Exactly one writer goroutine per connection (the write pump); all
  other goroutines send outbound data via a bounded channel
  (Invariant 7).
- Presence/cursor messages are explicitly excluded from the durable
  operation log (Invariant 9) and may be dropped/coalesced under load.

## To Be Defined in Phase 5

For each message type: exact payload schema, validation rules, retry
behavior, and idempotency behavior. Also: maximum message size, read/
write deadlines, ping interval, pong timeout, and the policy for slow
consumers and duplicate connections from the same client (see open
items in `docs/failure-model.md`, rows 10 and 12).
