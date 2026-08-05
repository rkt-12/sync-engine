// TypeScript counterpart to internal/protocol.

export const CURRENT_PROTOCOL_VERSION = 1;

export type MessageType =
  | "join_document"
  | "initial_sync"
  | "operation"
  | "operation_batch"
  | "acknowledgement"
  | "sync_request"
  | "sync_response"
  | "presence_update"
  | "cursor_update"
  | "error"
  | "ping"
  | "pong";

export interface Envelope {
  type: MessageType;
  protocolVersion: number;
  documentId?: string;
  clientId?: string;
  messageId?: string;
  payload?: unknown;
}

export type JoinDocumentPayload = Record<string, never>;

export interface InitialSyncPayload {
  operations: unknown[]; // each a canonical crdt Operation JSON object
  serverSequence: number;
}

export interface AcknowledgementPayload {
  acknowledgedMessageId: string;
  serverSequence: number;
}

export interface SyncRequestPayload {
  lastKnownServerSequence: number;
}

export interface SyncResponsePayload {
  operations: unknown[];
  serverSequence: number;
  hasMore: boolean;
}

export type PresenceStatus = "joined" | "left";

export interface PresenceUpdatePayload {
  status: PresenceStatus;
}

export interface CursorUpdatePayload {
  afterElementIdClient: number;
  afterElementIdCounter: number;
}

export interface ErrorPayload {
  code: string;
  message: string;
}