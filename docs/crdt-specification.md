# CRDT Specification

This document is the language-independent source of truth for the
sequence CRDT. Both the Go implementation (`internal/crdt`) and the
TypeScript client implementation must conform to this spec exactly.
Divergence between implementations is a bug in the implementation, not
in the spec.

## Algorithm

RGA (Replicated Growable Array), as described conceptually by Roh et
al. Each element is a node with a stable identity that is inserted
relative to a parent (left-origin) element, never at a numeric index.
Deletion is tombstoning, never physical removal.

## Identifier

```
Identifier {
    ClientID: uint64
    Counter:  uint64   // this client's logical clock value at creation
}
```

Total ordering, used only to break ties among concurrent siblings:

```
A > B  iff
    A.Counter > B.Counter
    OR (A.Counter == B.Counter AND A.ClientID > B.ClientID)
```

This ordering is a pure function of the identifier's own fields. It
never depends on arrival order, wall-clock time, database sequence
numbers, or map iteration order.

A sentinel `Identifier{ClientID: 0, Counter: 0}` is reserved to mean
"the virtual head of the document" and is used as `ParentElementID`
for insertions at the very start of the document. `ClientID: 0` is
never assigned to a real client.

## OperationID vs ElementID

Both are the `Identifier` type. For `InsertOperation`, `OperationID`
and the created element's `ElementID` are the same value — one causal
event, one identity. For `DeleteOperation`, `OperationID` is a fresh
identifier (needed for operation-level dedup); `TargetElementID` is a
reference to an existing element's identity and is a different value.

Distinct named Go types (`type ElementID Identifier`,
`type OperationID Identifier`) are used so the compiler distinguishes
them even though the underlying shape is identical.

## Operations

```
InsertOperation {
    OperationID       Identifier   // == ElementID
    DocumentID        string
    ClientID          uint64
    LogicalClock      uint64
    ParentElementID   Identifier   // sentinel = start of document
    Value             rune         // single character
}

DeleteOperation {
    OperationID       Identifier
    DocumentID        string
    ClientID          uint64
    LogicalClock      uint64
    TargetElementID   Identifier
}
```

All operations are immutable, deterministic, idempotent to apply, and
serializable/replayable.

## Element State (internal, not wire format)

```
Element {
    ID         Identifier
    ParentID   Identifier
    Value      rune
    Deleted    bool     // tombstone flag
}
```

## Insertion Ordering Rule

When inserting a new element E with `ParentElementID = P`:

1. Locate P in the current structure (or the virtual head if P is the
   sentinel).
2. Walk forward through P's existing children (elements whose
   `ParentID == P`, in their current sequence order) while the current
   child's `ID > E.ID`.
3. Insert E immediately before the first child encountered whose
   `ID < E.ID`, or at the end of P's children if none is smaller, or
   immediately after P if P currently has no children.

This means children of the same parent are always kept in descending
`Identifier` order relative to each other, and this ordering question
is answerable identically by any replica regardless of what order the
two concurrent inserts were received in — it depends only on comparing
`Identifier` values, both of which are already fully known once each
operation itself has arrived.

## Deletion Rule

`DeleteOperation.TargetElementID` marks the referenced element's
`Deleted` flag `true`. The element is never removed from the
structure — it remains as a stable anchor for any other element whose
`ParentElementID` points at it.

### Delete-before-insert

If a `DeleteOperation` arrives referencing an `ElementID` not yet
present locally, the target ID is recorded in a pending-tombstone set.
When the corresponding `InsertOperation` later arrives, the new
element is inserted as normal and immediately checked against the
pending-tombstone set; if present, it is marked deleted at the moment
of insertion. Final state is identical regardless of which operation
arrives first.

## Deduplication (CRDT layer)

Every replica maintains the set of `OperationID`s it has already
applied (`HasOperation`). `Apply(op)` is a no-op if
`HasOperation(op.OperationID)` is already true. This is what makes
`Apply` idempotent independent of any network- or database-layer
deduplication (see `docs/invariants.md`, Invariant 1).

## Materialization

`Materialize()` produces the visible document string by walking the
full element structure in its maintained order and concatenating the
`Value` of every element where `Deleted == false`. Tombstoned elements
are skipped but not removed from the underlying structure.

## Logical Clock

Each client maintains one monotonically increasing counter.

- On generating a local operation: increment the counter, stamp the
  operation's `LogicalClock` (and `Identifier.Counter` for inserts)
  with the new value.
- On receiving a remote operation: set local counter to
  `max(local counter, remote operation's LogicalClock) `, then treat
  as already incremented for the next local operation (i.e.
  `local = max(local, remote) `, next local op uses `local + 1`).
  This is the standard Lamport clock update rule and guarantees every
  operation ID a client generates is unique and every replica's clock
  eventually catches up to the highest clock value it has seen.

Logical clocks are used only for identifier uniqueness and tie-break
ordering — never as a proxy for "real" chronological time, and never
exposed to users as such.

## Serialization / Canonical Form

Wire and storage representation of an operation is a fixed-field JSON
object (exact field names and casing to be finalized in
`docs/websocket-protocol.md`); both Go and TypeScript implementations
must produce byte-identical JSON for the same logical operation before
computing anything content-addressed (e.g. test vector hashes), which
means: fixed key order, no locale-dependent number formatting,
UTF-8 text, and `Identifier` serialized as a two-field object
(`clientId`, `counter`), never as a concatenated string.

## Cross-Language Test Vectors

`testdata/*.json` files define scenarios (concurrent insertions,
delete-before-insert, duplicate operations, reordered operations,
offline merge) as: initial state, a list of operations with an
explicit application order per replica, and the expected final
materialized string. Both the Go test suite and the TypeScript test
suite consume the same files and assert the same final string. Any
change to CRDT semantics must be reflected here first.
