import { Identifier } from "./identifier";
import { DeleteOperation, ElementID, InsertOperation, Operation, OperationID, OperationKind } from "./operation";

// TypeScript counterpart to internal/crdt/serialization.go.

interface WireIdentifier {
  clientId: number;
  counter: number;
}

interface WireOperation {
  kind: OperationKind;
  operationId: WireIdentifier;
  documentId: string;
  clientId: number;
  logicalClock: number;
  parentElementId?: WireIdentifier;
  elementId?: WireIdentifier;
  value?: string;
  targetElementId?: WireIdentifier;
}

export function marshalOperation(op: Operation): string {
  if (op.kind === "insert") {
    const wire: WireOperation = {
      kind: "insert",
      operationId: op.operationId,
      documentId: op.documentId,
      clientId: op.clientId,
      logicalClock: op.logicalClock,
      parentElementId: op.parentElementId,
      elementId: op.elementId,
      value: op.value,
    };
    return JSON.stringify(wire);
  }

  const wire: WireOperation = {
    kind: "delete",
    operationId: op.operationId,
    documentId: op.documentId,
    clientId: op.clientId,
    logicalClock: op.logicalClock,
    targetElementId: op.targetElementId,
  };
  return JSON.stringify(wire);
}

export function unmarshalOperation(json: string): Operation {
  let wire: WireOperation;
  try {
    wire = JSON.parse(json) as WireOperation;
  } catch (err) {
    throw new Error(`crdt: unmarshalOperation: invalid JSON: ${(err as Error).message}`);
  }

  const operationId: OperationID = wire.operationId as Identifier;

  switch (wire.kind) {
    case "insert": {
      if (!wire.parentElementId) {
        throw new Error("crdt: unmarshalOperation: insert operation missing parentElementId");
      }
      if (!wire.elementId) {
        throw new Error("crdt: unmarshalOperation: insert operation missing elementId");
      }
      if (wire.value === undefined) {
        throw new Error("crdt: unmarshalOperation: insert operation missing value");
      }
      const codePoints = Array.from(wire.value);
      if (codePoints.length !== 1) {
        throw new Error(
          `crdt: unmarshalOperation: value must be exactly one character, got ${JSON.stringify(wire.value)}`,
        );
      }

      const insertOp: InsertOperation = {
        kind: "insert",
        operationId,
        documentId: wire.documentId,
        clientId: wire.clientId,
        logicalClock: wire.logicalClock,
        parentElementId: wire.parentElementId as ElementID,
        elementId: wire.elementId as ElementID,
        value: wire.value,
      };
      return insertOp;
    }

    case "delete": {
      if (!wire.targetElementId) {
        throw new Error("crdt: unmarshalOperation: delete operation missing targetElementId");
      }
      const deleteOp: DeleteOperation = {
        kind: "delete",
        operationId,
        documentId: wire.documentId,
        clientId: wire.clientId,
        logicalClock: wire.logicalClock,
        targetElementId: wire.targetElementId as ElementID,
      };
      return deleteOp;
    }

    default:
      throw new Error(`crdt: unmarshalOperation: unknown kind ${JSON.stringify(wire.kind)}`);
  }
}