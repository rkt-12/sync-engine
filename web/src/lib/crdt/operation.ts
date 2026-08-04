import { Identifier } from "./identifier";

//  * TypeScript counterpart to internal/crdt/operation.go.

export type ElementID = Identifier;
export type OperationID = Identifier;

export type OperationKind = "insert" | "delete";

export interface InsertOperation {
  readonly kind: "insert";
  readonly operationId: OperationID;
  readonly documentId: string;
  readonly clientId: number;
  readonly logicalClock: number;
  readonly parentElementId: ElementID;
  readonly elementId: ElementID;
  readonly value: string;
}

export interface DeleteOperation {
  readonly kind: "delete";
  readonly operationId: OperationID;
  readonly documentId: string;
  readonly clientId: number;
  readonly logicalClock: number;
  readonly targetElementId: ElementID;
}

export type Operation = InsertOperation | DeleteOperation;