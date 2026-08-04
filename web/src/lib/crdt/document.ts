import { ElementID, InsertOperation, DeleteOperation, Operation, OperationID } from "./operation";
import { Identifier, RootID, isGreater, isRoot } from "./identifier";

// TypeScript counterpart to internal/crdt/document.go.

interface ElementNode {
  readonly id: ElementID;
  readonly parent: ElementID;
  readonly value: string;
  deleted: boolean;
}

function keyOf(id: Identifier): string {
  return `${id.clientId}:${id.counter}`;
}

export class Document {
  private readonly id: string;
  private readonly elements = new Map<string, ElementNode>();
  private readonly children = new Map<string, ElementID[]>();
  private readonly appliedOps = new Set<string>();
  private readonly pendingTombstones = new Set<string>();

  constructor(documentId: string) {
    this.id = documentId;
  }

  get documentId(): string {
    return this.id;
  }

  hasOperation(id: OperationID): boolean {
    return this.appliedOps.has(keyOf(id));
  }

  insert(op: InsertOperation): void {
    if (op.documentId !== this.id) {
      throw new Error(
        `crdt: operation is for document "${op.documentId}", this document is "${this.id}"`,
      );
    }
    if (this.hasOperation(op.operationId)) {
      return;
    }

    const elementKey = keyOf(op.elementId);
    if (this.elements.has(elementKey)) {
      throw new Error(`crdt: element already exists: ${elementKey}`);
    }
    if (!isRoot(op.parentElementId) && !this.elements.has(keyOf(op.parentElementId))) {
      throw new Error(`crdt: parent element not found: ${keyOf(op.parentElementId)}`);
    }

    const node: ElementNode = {
      id: op.elementId,
      parent: op.parentElementId,
      value: op.value,
      deleted: false,
    };
    this.elements.set(elementKey, node);
    this.insertSorted(op.parentElementId, op.elementId);

    if (this.pendingTombstones.has(elementKey)) {
      node.deleted = true;
      this.pendingTombstones.delete(elementKey);
    }

    this.appliedOps.add(keyOf(op.operationId));
  }

  delete(op: DeleteOperation): void {
    if (op.documentId !== this.id) {
      throw new Error(
        `crdt: operation is for document "${op.documentId}", this document is "${this.id}"`,
      );
    }
    if (this.hasOperation(op.operationId)) {
      return;
    }

    const targetKey = keyOf(op.targetElementId);
    const existing = this.elements.get(targetKey);
    if (existing) {
      existing.deleted = true;
    } else {
      this.pendingTombstones.add(targetKey);
    }

    this.appliedOps.add(keyOf(op.operationId));
  }

  apply(op: Operation): void {
    if (op.kind === "insert") {
      this.insert(op);
    } else {
      this.delete(op);
    }
  }

  applyBatch(ops: readonly Operation[]): void {
    for (let i = 0; i < ops.length; i++) {
      try {
        this.apply(ops[i]);
      } catch (err) {
        throw new Error(
          `applying operation ${i + 1} of ${ops.length}: ${(err as Error).message}`,
        );
      }
    }
  }

  materialize(): string {
    let out = "";
    this.walk(keyOf(RootID), (_id, el) => {
      if (!el.deleted) {
        out += el.value;
      }
    });
    return out;
  }

  visibleSequence(): ElementID[] {
    const out: ElementID[] = [];
    this.walk(keyOf(RootID), (id, el) => {
      if (!el.deleted) {
        out.push(id);
      }
    });
    return out;
  }

  private walk(parentKey: string, visit: (id: ElementID, el: ElementNode) => void): void {
    const siblings = this.children.get(parentKey);
    if (!siblings) {
      return;
    }
    for (const childId of siblings) {
      const childKey = keyOf(childId);
      const el = this.elements.get(childKey);
      if (!el) {
        throw new Error(`crdt: internal invariant violated: child ${childKey} has no element entry`);
      }
      visit(childId, el);
      this.walk(childKey, visit);
    }
  }

  private insertSorted(parentId: ElementID, childId: ElementID): void {
    const parentKey = keyOf(parentId);
    const siblings = this.children.get(parentKey) ?? [];

    let idx = 0;
    while (idx < siblings.length && isGreater(siblings[idx], childId)) {
      idx++;
    }
    siblings.splice(idx, 0, childId);
    this.children.set(parentKey, siblings);
  }
}