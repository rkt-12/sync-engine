# Architecture

## Overview

The sync engine is a modular monolith: a single Go server process handles
WebSocket connections, CRDT application, and PostgreSQL persistence for
many concurrently-edited documents. Clients (Next.js/TypeScript) hold a
full local CRDT replica and edit locally before any round-trip to the
server (local-first editing).

There is intentionally no Kafka, Redis, Kubernetes, or multi-server
topology in v1. The system is a star topology: all clients synchronize
through one server; clients never sync directly with each other.

## Component Diagram

```
Next.js Client
       |
       | WebSocket (JSON protocol, versioned)
       v
Go Synchronization Server
       |
       +----------------------+
       | Connection Manager   |  owns: live ws connections, read/write pumps
       +----------------------+
       | Room Manager         |  owns: DocumentRoom lifecycle, lookup by documentId
       +----------------------+
       | Document Rooms       |  owns: set of clients subscribed to one document,
       |                       |        broadcast fan-out
       +----------------------+
       | CRDT Engine          |  pure, no I/O, no network, no DB — internal/crdt
       +----------------------+
       | Sync Protocol        |  message envelope validation, dedup, ack tracking
       +----------------------+
       | Snapshot Manager     |  periodic/triggered snapshot creation + restore
       +----------------------+
       | PostgreSQL Store     |  operation log, snapshots, documents
       +----------------------+
                  |
                  v
              PostgreSQL
```

## Component Responsibilities

**Connection Manager**
Owns the set of live WebSocket connections. Registers/unregisters
connections, runs one read pump and one write pump goroutine per
connection, enforces read/write deadlines, ping/pong heartbeats, and
message size limits. Guarantees invariant 6 (at most one writer per
connection) by making the write pump the sole owner of the socket's
write side; all other goroutines send outbound messages via a bounded
channel, never by writing to the socket directly.

**Room Manager**
Maps `documentId -> DocumentRoom`, creating rooms lazily on first join
and tearing them down when the last client leaves (with a grace period
to avoid churn on quick reconnects).

**Document Room**
Coordinates all clients currently viewing one document: fan-out
broadcast of accepted operations, presence state for that document,
and the in-memory CRDT replica used to serve `initial_sync` /
`sync_response` without hitting Postgres for every read.

**CRDT Engine (`internal/crdt`)**
Pure Go package. No HTTP, no WebSocket, no PostgreSQL, no server state.
Implements the RGA-based sequence CRDT: identifiers, operations, apply
logic, materialization, serialization. Independently unit-testable and
shared conceptually (not literally — separate implementation) with the
TypeScript client via common test vectors.

**Sync Protocol**
Validates incoming message envelopes against the versioned wire
protocol, applies message-layer deduplication (by `messageId`), and
tracks acknowledgement state per connection.

**Snapshot Manager**
Creates periodic snapshots of a document's CRDT state plus the highest
included Postgres `sequence_id`, and restores state on server startup
or document room creation by loading the latest snapshot and replaying
operations after it.

**PostgreSQL Store**
Durable operation log (source of truth) and snapshot storage
(optimization only). Never stores mutable "current text" as a column.

## Data Flow: A Single Edit

1. User types in the editor.
2. Client CRDT engine generates an `InsertOperation`, applies it to the
   local replica immediately (local-first), and renders.
3. Operation is placed in a local pending-operations queue and sent
   over the WebSocket.
4. Server validates the message envelope, checks the CRDT layer's
   `HasOperation` for dedup, persists to PostgreSQL
   (`INSERT ... ON CONFLICT DO NOTHING` on `operation_id`), and applies
   it to the server's in-memory replica for that document room.
5. Server broadcasts the operation to all other connected clients in
   the room, and sends an acknowledgement back to the originating
   client.
6. Originating client removes the operation from its pending queue on
   ACK. Other clients apply the remote operation to their local
   replicas (idempotently).

## Why This Shape

- Keeping the CRDT engine dependency-free means it can be fuzzed,
  benchmarked, and proven correct in complete isolation from
  networking or database flakiness — this is deliberate, see
  `docs/invariants.md` and the simulation framework.
- The server is a privileged replica (durability + fan-out) but not a
  conflict-resolution authority — resolution logic is identical
  wherever it runs, which is what makes cross-language compatibility
  (Go server / TypeScript client) meaningful rather than aspirational.
- Single-process, single-database in v1 avoids distributed-systems
  problems (multi-node consensus, cross-node partitions) that are not
  the point of this project. The point is proving CRDT convergence
  under an adversarial *message-delivery* model, not building a
  distributed database.
