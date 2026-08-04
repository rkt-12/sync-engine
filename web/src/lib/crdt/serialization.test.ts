import { describe, expect, it } from "vitest";
import { RootID } from "./identifier";
import { DeleteOperation, InsertOperation } from "./operation";
import { marshalOperation, unmarshalOperation } from "./serialization";

function insertOp(): InsertOperation {
  return {
    kind: "insert",
    operationId: { clientId: 1, counter: 5 },
    documentId: "doc-1",
    clientId: 1,
    logicalClock: 5,
    parentElementId: RootID,
    elementId: { clientId: 1, counter: 5 },
    value: "a",
  };
}

function deleteOp(): DeleteOperation {
  return {
    kind: "delete",
    operationId: { clientId: 2, counter: 9 },
    documentId: "doc-1",
    clientId: 2,
    logicalClock: 9,
    targetElementId: { clientId: 1, counter: 5 },
  };
}

describe("marshal/unmarshal round trips", () => {
  it("round-trips an insert operation", () => {
    const original = insertOp();
    const json = marshalOperation(original);
    const decoded = unmarshalOperation(json);
    expect(decoded).toEqual(original);
  });

  it("round-trips a delete operation", () => {
    const original = deleteOp();
    const json = marshalOperation(original);
    const decoded = unmarshalOperation(json);
    expect(decoded).toEqual(original);
  });

  it("omits targetElementId for an insert", () => {
    const json = marshalOperation(insertOp());
    expect(json).not.toContain("targetElementId");
  });

  it("omits insert-only fields for a delete", () => {
    const json = marshalOperation(deleteOp());
    for (const field of ["parentElementId", "elementId", "value"]) {
      expect(json).not.toContain(field);
    }
  });
});

describe("unmarshalOperation validation", () => {
  it("rejects an unknown kind", () => {
    const input = JSON.stringify({
      kind: "replace",
      operationId: { clientId: 1, counter: 1 },
      documentId: "doc-1",
      clientId: 1,
      logicalClock: 1,
    });
    expect(() => unmarshalOperation(input)).toThrow();
  });

  it("rejects an insert missing elementId", () => {
    const input = JSON.stringify({
      kind: "insert",
      operationId: { clientId: 1, counter: 1 },
      documentId: "doc-1",
      clientId: 1,
      logicalClock: 1,
      parentElementId: { clientId: 0, counter: 0 },
      value: "a",
    });
    expect(() => unmarshalOperation(input)).toThrow();
  });

  it("rejects a delete missing targetElementId", () => {
    const input = JSON.stringify({
      kind: "delete",
      operationId: { clientId: 1, counter: 1 },
      documentId: "doc-1",
      clientId: 1,
      logicalClock: 1,
    });
    expect(() => unmarshalOperation(input)).toThrow();
  });

  it.each([
    ["empty string", "", true],
    ["two ascii characters", "ab", true],
    ["single ascii character", "a", false],
    ["single multibyte character", "é", false],
    ["single emoji (surrogate pair in UTF-16)", "🙂", false],
  ])("value %s -> wantErr=%s", (_name, value, wantErr) => {
    const input = JSON.stringify({
      kind: "insert",
      operationId: { clientId: 1, counter: 1 },
      documentId: "doc-1",
      clientId: 1,
      logicalClock: 1,
      parentElementId: { clientId: 0, counter: 0 },
      elementId: { clientId: 1, counter: 1 },
      value,
    });
    if (wantErr) {
      expect(() => unmarshalOperation(input)).toThrow();
    } else {
      expect(() => unmarshalOperation(input)).not.toThrow();
    }
  });

  it("rejects invalid JSON", () => {
    expect(() => unmarshalOperation("not json")).toThrow();
  });
});

describe("canonical field order", () => {
  it("matches the Go implementation's pinned JSON exactly", () => {
    // This exact string is pinned in internal/crdt/serialization_test.go
    // (TestMarshalOperation_FieldOrderIsStable) for the equivalent Go
    // operation. If this test and the Go test ever disagree, the wire
    // formats have diverged -- this is the cheapest possible check for
    // that, no server round trip required.
    const op: InsertOperation = {
      kind: "insert",
      operationId: { clientId: 1, counter: 1 },
      documentId: "doc-1",
      clientId: 1,
      logicalClock: 1,
      parentElementId: RootID,
      elementId: { clientId: 1, counter: 1 },
      value: "a",
    };

    const want =
      '{"kind":"insert","operationId":{"clientId":1,"counter":1},"documentId":"doc-1",' +
      '"clientId":1,"logicalClock":1,"parentElementId":{"clientId":0,"counter":0},' +
      '"elementId":{"clientId":1,"counter":1},"value":"a"}';

    expect(marshalOperation(op)).toBe(want);
  });
});