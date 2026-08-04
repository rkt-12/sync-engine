// TypeScript counterpart to internal/crdt/identifier.go. 
// Identifier is a globally-unique, deterministically-orderable identity used for both CRDT elements and operations.

export interface Identifier {
  readonly clientId: number;
  readonly counter: number;
}

const ZERO_CLIENT_ID = 0;

export const RootID: Identifier = Object.freeze({
  clientId: ZERO_CLIENT_ID,
  counter: 0,
});

export function isRoot(id: Identifier): boolean {
  return id.clientId === RootID.clientId && id.counter === RootID.counter;
}

export function isLess(a: Identifier, b: Identifier): boolean {
  if (a.counter !== b.counter) {
    return a.counter < b.counter;
  }
  return a.clientId < b.clientId;
}

export function isGreater(a: Identifier, b: Identifier): boolean {
  return isLess(b, a);
}

export function identifiersEqual(a: Identifier, b: Identifier): boolean {
  return a.clientId === b.clientId && a.counter === b.counter;
}