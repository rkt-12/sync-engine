# Synchronization / Reconnection Protocol

Status: STUB — full state-machine specification is a Phase 7
deliverable. This file records the decisions already made in Phase 0.

## Decided in Phase 0

**Approach: server-sequence-offset catch-up, not a version vector.**

The system topology is star-shaped: clients only ever synchronize
through the single Go server, never directly with each other. In that
topology, "give me everything after server sequence N" is sufficient
and correct, because the server's Postgres `sequence_id` is a
strictly totally ordered, complete record of every operation ever
accepted for a document — order-independent because CRDT correctness
never depends on that log's order in the first place (see
`docs/crdt-specification.md`).

A version vector (one counter per client, compared pairwise) is the
textbook-correct answer for general peer-to-peer sync, but adds
complexity (vector growth, garbage collection for clients that never
return) unjustified for a star topology. It is noted here as future
work if this project ever grows a peer-to-peer or multi-server mode.

### Reconnect flow

1. Client reconnects, sends `sync_request{lastKnownServerSeq}`.
2. Server responds with `sync_response` containing all operations
   where `sequence_id > lastKnownServerSeq`, paginated if large.
3. Client applies received operations to its local replica
   (idempotent per Invariant 1 — safe even on overlap).
4. Client flushes its local pending-operations queue (edits made while
   offline) to the server.
5. Server persists and broadcasts these to other connected clients;
   the originating client's `lastKnownServerSeq` is only advanced once
   an operation is durably acknowledged — never optimistically.

### The three-way race

The scenario where (a) the server is sending missing operations,
(b) the client has unsent offline operations, and (c) new live
operations are being generated simultaneously is safe by construction:
CRDT correctness never depends on the order operations are applied in
(Invariant 2, Invariant 4). The only operational property that must
hold is that step 4 above is never silently lost, and
`lastKnownServerSeq` is never advanced past an operation the client
hasn't durably received — both are protocol/bookkeeping concerns, not
CRDT-correctness concerns.

## To Be Defined in Phase 7

Exact client-side state machine (states: disconnected, syncing,
flushing pending, live), exponential-backoff-with-jitter parameters,
IndexedDB schema for the offline pending-operations queue, and
handling of `sync_response` pagination for large catch-up sets.
