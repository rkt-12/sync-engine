# Failure Model

This document defines the adversarial conditions the system is
designed to survive without corrupting convergence (see
`docs/invariants.md`), and explicitly names what is out of scope for
v1.

## Assumptions

- Transport is WebSocket over TCP: messages that are delivered arrive
  uncorrupted and are not partially delivered. TCP does not protect
  us from application-level delay, reordering (across separate
  reconnect/retry attempts), duplication (via retries), or loss
  (connection drop before delivery).
- No trusted global clock exists across clients and server. All
  ordering used for correctness is logical (see
  `docs/crdt-specification.md`), never wall-clock.
- Single Go server process, single PostgreSQL instance in v1. No
  multi-node server topology.

## Failures the System Must Survive

| # | Failure | Required behavior |
|---|---|---|
| 1 | Message delay (arbitrary, unbounded) | Convergence still holds once the message eventually arrives |
| 2 | Message reordering | Convergence independent of arrival order |
| 3 | Message duplication (e.g. client retry before ACK) | Idempotent application, no duplicate visible characters |
| 4 | Message loss (permanent) | Recovered via reconnect/anti-entropy sync, not raw redelivery |
| 5 | Client disconnects mid-edit | Local edits preserved, resent on reconnect |
| 6 | Client offline for extended periods | Offline edits queued locally, merged correctly on reconnect |
| 7 | Server restart | No acknowledged operation is lost (Invariant 3) |
| 8 | Concurrent edits at the identical logical position | Deterministic tie-break, same result on every replica |
| 9 | Delete arrives before its corresponding Insert | Resolved correctly once the Insert arrives (tombstone-pending) |
| 10 | Duplicate WebSocket connections from the same client | Handled gracefully — exact policy (reject new / drop old) finalized in `docs/websocket-protocol.md` |
| 11 | Client reconnects from a stale/very old server offset | Full catch-up via paginated `sync_response`, not silently dropped |
| 12 | Slow consumer (client not draining fast enough) | Bounded outbound channel; policy for what happens when full (drop vs. disconnect) finalized in `docs/websocket-protocol.md` |
| 13 | Malformed or oversized incoming message | Rejected safely with an `error` message; connection not corrupted |

## Explicitly Out of Scope for v1

- **Byzantine clients.** We do not defend against clients forging
  operations, claiming another client's `ClientID`, or sending
  intentionally malicious payloads beyond basic size/format
  validation. Authentication/authorization is explicitly deferred
  (see Section 24 of the original project brief) and is not a
  correctness mechanism for the CRDT layer.
- **Multi-node server partitions.** There is one server process. We do
  not model split-brain between multiple synchronization servers.
  If this project grows to multiple server nodes, this is future
  work requiring a version-vector-based sync protocol (see
  `docs/synchronization-protocol.md` for why we chose the simpler
  offset-based approach for a star topology in v1).
- **PostgreSQL data corruption or storage-layer failures** beyond
  what standard transactional guarantees provide.
- **Network partitions that never heal.** We assume any partition is
  eventually resolved (client eventually reconnects); we do not
  guarantee anything about a permanently unreachable client beyond
  "it never converges because it never receives the operations."

## Relationship to Testing

Every row in the "must survive" table above must have a corresponding
scenario in the distributed-system simulator (`internal/simulation`)
and, where applicable, a named test vector in `testdata/`. A failure
mode with no corresponding test is considered undocumented risk, not
a verified guarantee.
