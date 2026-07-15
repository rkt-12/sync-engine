# System Invariants

These invariants are the north star for every design and implementation
decision in this project. Every non-trivial change should be checked
against this list, and PRs/commits touching CRDT, sync, or persistence
logic should reference which invariant(s) they protect or could
threaten.

---

**INVARIANT 1 — Idempotent application**
Applying the same operation multiple times does not alter the final
state.
`Apply(Apply(S, op), op) == Apply(S, op)`

**INVARIANT 2 — Strong eventual consistency**
Replicas that have received the same set of valid operations
eventually materialize identical documents, regardless of the order
in which those operations were received.
`Materialize(R1) == Materialize(R2) == ... == Materialize(RN)`

**INVARIANT 3 — No lost acknowledged operations**
No operation the server has acknowledged is ever lost, including
across a server restart.

**INVARIANT 4 — Order-independent conflict resolution**
CRDT conflict resolution never depends on operation arrival order.

**INVARIANT 5 — No wall-clock dependence**
CRDT conflict resolution never depends on wall-clock time.

**INVARIANT 6 — Globally unique operation IDs**
A persisted `operation_id` is unique within a document.

**INVARIANT 7 — Single writer per connection**
A WebSocket connection has at most one concurrent writer goroutine.

**INVARIANT 8 — Snapshot/replay equivalence**
A snapshot plus all subsequent operations reconstructs the same state
as replaying the complete operation history.
`Materialize(Restore(snapshot) + Replay(opsAfter)) == Materialize(Replay(allOps))`

**INVARIANT 9 — Presence is not durable state**
Presence messages (cursor position, online status, selection range)
never affect durable document state and are never mixed into the
operation log.

**INVARIANT 10 — Safe offline merge**
Disconnected clients can submit valid offline operations after
reconnection without overwriting concurrent changes made by other
clients while they were offline.

**INVARIANT 11 — Stable references / tombstones**
An element, once inserted, is never physically removed from the CRDT
structure. Deletion is tombstoning (a visibility flag), because other
elements may reference it as a parent.

**INVARIANT 12 — Monotonic logical clocks**
Each client's logical clock is monotonically increasing and is updated
correctly on receipt of remote operations (`local = max(local, remote)`).
It never decreases.

---

## Explicitly Not Guaranteed (Known Limitations, v1)

- No defense against Byzantine/malicious clients forging operations.
- No multi-server / multi-node consensus — single Go server process,
  single PostgreSQL instance.
- No cross-document transactional guarantees.
- No guarantee on presence delivery (lossy by design, see Invariant 9).
