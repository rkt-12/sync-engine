import { describe, expect, it } from "vitest";
import { RootID } from "./identifier";
import { DeleteOperation, ElementID, InsertOperation, Operation } from "./operation";
import { Document } from "./document";

function insertOp(
  documentId: string,
  clientId: number,
  counter: number,
  parent: ElementID,
  value: string,
): InsertOperation {
  return {
    kind: "insert",
    operationId: { clientId, counter },
    documentId,
    clientId,
    logicalClock: counter,
    parentElementId: parent,
    elementId: { clientId, counter },
    value,
  };
}

function deleteOp(
  documentId: string,
  clientId: number,
  counter: number,
  target: ElementID,
): DeleteOperation {
  return {
    kind: "delete",
    operationId: { clientId, counter },
    documentId,
    clientId,
    logicalClock: counter,
    targetElementId: target,
  };
}

describe("Document.insert", () => {
  it("inserts a single character", () => {
    const d = new Document("doc-1");
    d.insert(insertOp("doc-1", 1, 1, RootID, "a"));
    expect(d.materialize()).toBe("a");
  });

  it("builds a sequential chain", () => {
    const d = new Document("doc-1");
    const a = insertOp("doc-1", 1, 1, RootID, "a");
    const b = insertOp("doc-1", 1, 2, a.elementId, "b");
    const c = insertOp("doc-1", 1, 3, b.elementId, "c");
    for (const op of [a, b, c]) d.insert(op);
    expect(d.materialize()).toBe("abc");
  });

  it("throws on an unknown parent", () => {
    const d = new Document("doc-1");
    const ghost = { clientId: 99, counter: 99 };
    expect(() => d.insert(insertOp("doc-1", 1, 1, ghost, "a"))).toThrow();
  });

  it("throws on a mismatched documentId", () => {
    const d = new Document("doc-1");
    expect(() => d.insert(insertOp("doc-2", 1, 1, RootID, "a"))).toThrow();
  });

  it("is idempotent", () => {
    const d = new Document("doc-1");
    const op = insertOp("doc-1", 1, 1, RootID, "a");
    d.insert(op);
    expect(() => d.insert(op)).not.toThrow();
    expect(d.materialize()).toBe("a");
  });
});

describe("Document.delete", () => {
  it("tombstones an existing element", () => {
    const d = new Document("doc-1");
    const a = insertOp("doc-1", 1, 1, RootID, "a");
    d.insert(a);
    d.delete(deleteOp("doc-1", 1, 2, a.elementId));
    expect(d.materialize()).toBe("");
  });

  it("is idempotent", () => {
    const d = new Document("doc-1");
    const a = insertOp("doc-1", 1, 1, RootID, "a");
    d.insert(a);
    const del = deleteOp("doc-1", 1, 2, a.elementId);
    d.delete(del);
    expect(() => d.delete(del)).not.toThrow();
    expect(d.materialize()).toBe("");
  });

  it("throws on a mismatched documentId", () => {
    const d = new Document("doc-1");
    expect(() => d.delete(deleteOp("doc-2", 1, 1, { clientId: 1, counter: 1 }))).toThrow();
  });

  it("handles delete-before-insert", () => {
    const d = new Document("doc-1");
    const a = insertOp("doc-1", 1, 1, RootID, "a");
    const del = deleteOp("doc-1", 2, 1, a.elementId);

    d.delete(del); // arrives first, target doesn't exist yet
    expect(d.materialize()).toBe("");

    d.insert(a); // now arrives -- should be pre-tombstoned
    expect(d.materialize()).toBe("");
  });

  it("delete-before-insert matches insert-then-delete", () => {
    const a = insertOp("doc-1", 1, 1, RootID, "a");
    const del = deleteOp("doc-1", 2, 1, a.elementId);

    const d1 = new Document("doc-1");
    d1.insert(a);
    d1.delete(del);

    const d2 = new Document("doc-1");
    d2.delete(del);
    d2.insert(a);

    expect(d1.materialize()).toBe(d2.materialize());
  });
});

describe("convergence", () => {
  it("resolves concurrent siblings identically regardless of arrival order", () => {
    // Same scenario as the Go test
    // (TestDocument_ConcurrentInsertion_SameParent_OrderIndependentConvergence):
    // x=(client 2, counter 1), y=(client 3, counter 1). Equal counters,
    // tie-break on clientId: y's clientId (3) > x's clientId (2), so y
    // sorts before x -- descending order puts y first.
    const a = insertOp("doc-1", 1, 1, RootID, "a");
    const x = insertOp("doc-1", 2, 1, a.elementId, "X");
    const y = insertOp("doc-1", 3, 1, a.elementId, "Y");

    const d1 = new Document("doc-1");
    d1.insert(a);
    d1.insert(x);
    d1.insert(y);

    const d2 = new Document("doc-1");
    d2.insert(a);
    d2.insert(y);
    d2.insert(x);

    expect(d1.materialize()).toBe(d2.materialize());
    expect(d1.materialize()).toBe("aYX");
  });

  it("converges under a causally-valid reordering with delete-before-insert", () => {
    const a = insertOp("doc-1", 1, 1, RootID, "a");
    const b = insertOp("doc-1", 1, 2, a.elementId, "b");
    const c = insertOp("doc-1", 1, 3, b.elementId, "c");
    const x = insertOp("doc-1", 2, 1, a.elementId, "X"); // sibling of b
    const delB = deleteOp("doc-1", 3, 1, b.elementId);

    const forward: Operation[] = [a, b, c, x, delB];
    // delB may move before its target (delete-before-insert); x has no
    // causal dependency on b/c and may move earlier; a must still
    // precede b, and b must still precede c.
    const reordered: Operation[] = [delB, a, x, b, c];

    const d1 = new Document("doc-1");
    d1.applyBatch(forward);

    const d2 = new Document("doc-1");
    d2.applyBatch(reordered);

    expect(d1.materialize()).toBe(d2.materialize());
  });

  it("is unaffected by duplicate operations", () => {
    const a = insertOp("doc-1", 1, 1, RootID, "a");
    const b = insertOp("doc-1", 1, 2, a.elementId, "b");
    const ops: Operation[] = [a, b, a, a, b];

    const d = new Document("doc-1");
    d.applyBatch(ops);
    expect(d.materialize()).toBe("ab");
  });
});

describe("hasOperation", () => {
  it("reflects whether an operation has been applied", () => {
    const d = new Document("doc-1");
    const op = insertOp("doc-1", 1, 1, RootID, "a");
    expect(d.hasOperation(op.operationId)).toBe(false);
    d.insert(op);
    expect(d.hasOperation(op.operationId)).toBe(true);
  });
});

describe("visibleSequence", () => {
  it("reflects document order and excludes tombstones", () => {
    const d = new Document("doc-1");
    const a = insertOp("doc-1", 1, 1, RootID, "a");
    const b = insertOp("doc-1", 1, 2, a.elementId, "b");
    d.insert(a);
    d.insert(b);
    d.delete(deleteOp("doc-1", 1, 3, a.elementId));

    const visible = d.visibleSequence();
    expect(visible).toHaveLength(1);
    expect(visible[0]).toEqual(b.elementId);
  });
});