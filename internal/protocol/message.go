package protocol

import "encoding/json"

// CurrentProtocolVersion is the protocol version this package implements.
const CurrentProtocolVersion = 1

type MessageType string

const (
	// TypeJoinDocument: client -> server. Requests to join a document's room
	TypeJoinDocument MessageType = "join_document"

	// TypeInitialSync: server -> client. Sent once, immediately after a successful join.
	TypeInitialSync MessageType = "initial_sync"

	// TypeOperation: client -> server, and server -> client (broadcast).
	// Carries exactly one CRDT operation, encoded via crdt.MarshalOperation as the Payload.
	TypeOperation MessageType = "operation"

	// TypeOperationBatch: client -> server. Carries multiple CRDT
	// operations at once -- e.g. a batch of offline edits flushed on reconnect.
	TypeOperationBatch MessageType = "operation_batch"

	// TypeAcknowledgement: server -> client. Confirms a client-sent operation
	TypeAcknowledgement MessageType = "acknowledgement"

	// TypeSyncRequest: client -> server. Requests catch-up: "send me everything after this server sequence"
	TypeSyncRequest MessageType = "sync_request"

	// TypeSyncResponse: server -> client. Answers a sync_request
	TypeSyncResponse MessageType = "sync_response"

	// TypePresenceUpdate: bidirectional. A client's online/offline status within a document room.
	TypePresenceUpdate MessageType = "presence_update"

	// TypeCursorUpdate: bidirectional. A client's cursor/selection position.
	TypeCursorUpdate MessageType = "cursor_update"

	// TypeError: server -> client (primarily). Reports a rejected or malformed message.
	TypeError MessageType = "error"

	// TypePing / TypePong: bidirectional heartbeat, no payload.
	TypePing MessageType = "ping"
	TypePong MessageType = "pong"
)

// Envelope is the single outer wire structure for every message in both directions.
type Envelope struct {
	Type            MessageType     `json:"type"`
	ProtocolVersion int             `json:"protocolVersion"`
	DocumentID      string          `json:"documentId,omitempty"`
	ClientID        string          `json:"clientId,omitempty"`
	MessageID       string          `json:"messageId,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

// JoinDocumentPayload is the payload for TypeJoinDocument.
type JoinDocumentPayload struct{}

// InitialSyncPayload is the payload for TypeInitialSync.
type InitialSyncPayload struct {
	Operations     []json.RawMessage `json:"operations"`
	ServerSequence int64             `json:"serverSequence"`
}

// AcknowledgementPayload is the payload for TypeAcknowledgement.
type AcknowledgementPayload struct {
	AcknowledgedMessageID string `json:"acknowledgedMessageId"`
	ServerSequence        int64  `json:"serverSequence"`
}

// SyncRequestPayload is the payload for TypeSyncRequest.
type SyncRequestPayload struct {
	LastKnownServerSequence int64 `json:"lastKnownServerSequence"`
}

// SyncResponsePayload is the payload for TypeSyncResponse.
type SyncResponsePayload struct {
	Operations     []json.RawMessage `json:"operations"`
	ServerSequence int64             `json:"serverSequence"`
	HasMore        bool              `json:"hasMore"`
}

// PresenceUpdatePayload is the payload for TypePresenceUpdate.
type PresenceUpdatePayload struct {
	Status PresenceStatus `json:"status"`
}

// PresenceStatus enumerates PresenceUpdatePayload.Status values.
type PresenceStatus string

const (
	PresenceJoined PresenceStatus = "joined"
	PresenceLeft   PresenceStatus = "left"
)

// CursorUpdatePayload is the payload for TypeCursorUpdate.
type CursorUpdatePayload struct {
	AfterElementIDClient  uint64 `json:"afterElementIdClient"`
	AfterElementIDCounter uint64 `json:"afterElementIdCounter"`
}

// ErrorPayload is the payload for TypeError.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
