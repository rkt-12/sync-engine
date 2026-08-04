export { RootID, isRoot, isLess, isGreater, identifiersEqual } from "./identifier";
export type { Identifier } from "./identifier";

export { LogicalClock } from "./clock";

export type { ElementID, OperationID, OperationKind, InsertOperation, DeleteOperation, Operation } from "./operation";

export { marshalOperation, unmarshalOperation } from "./serialization";

export { Document } from "./document";