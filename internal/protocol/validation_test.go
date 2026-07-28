package protocol

import "testing"

func TestValidateEnvelope_UnsupportedProtocolVersion(t *testing.T) {
	env := Envelope{Type: TypePing, ProtocolVersion: 99}
	if err := ValidateEnvelope(env); err == nil {
		t.Fatal("expected an error for unsupported protocol version, got nil")
	}
}

func TestValidateEnvelope_UnknownMessageType(t *testing.T) {
	env := Envelope{Type: "not_a_real_type", ProtocolVersion: CurrentProtocolVersion}
	if err := ValidateEnvelope(env); err == nil {
		t.Fatal("expected an error for unknown message type, got nil")
	}
}

func TestValidateEnvelope_PingPong_NoFieldsRequired(t *testing.T) {
	for _, typ := range []MessageType{TypePing, TypePong} {
		env := Envelope{Type: typ, ProtocolVersion: CurrentProtocolVersion}
		if err := ValidateEnvelope(env); err != nil {
			t.Errorf("%s: expected no error for a bare envelope, got %v", typ, err)
		}
	}
}

func TestValidateEnvelope_RequiredFieldsPerType(t *testing.T) {
	payload := []byte(`{"x":1}`)

	tests := []struct {
		name    string
		env     Envelope
		wantErr bool
	}{
		{
			name:    "join_document missing clientId",
			env:     Envelope{Type: TypeJoinDocument, ProtocolVersion: CurrentProtocolVersion, DocumentID: "doc-1"},
			wantErr: true,
		},
		{
			name: "join_document with all required fields",
			env: Envelope{Type: TypeJoinDocument, ProtocolVersion: CurrentProtocolVersion,
				DocumentID: "doc-1", ClientID: "client-1"},
			wantErr: false,
		},
		{
			name:    "operation missing messageId",
			env:     Envelope{Type: TypeOperation, ProtocolVersion: CurrentProtocolVersion, DocumentID: "doc-1", ClientID: "client-1", Payload: payload},
			wantErr: true,
		},
		{
			name: "operation with all required fields",
			env: Envelope{Type: TypeOperation, ProtocolVersion: CurrentProtocolVersion,
				DocumentID: "doc-1", ClientID: "client-1", MessageID: "msg-1", Payload: payload},
			wantErr: false,
		},
		{
			name:    "operation_batch missing payload",
			env:     Envelope{Type: TypeOperationBatch, ProtocolVersion: CurrentProtocolVersion, DocumentID: "doc-1", ClientID: "client-1", MessageID: "msg-1"},
			wantErr: true,
		},
		{
			name:    "acknowledgement missing messageId",
			env:     Envelope{Type: TypeAcknowledgement, ProtocolVersion: CurrentProtocolVersion, Payload: payload},
			wantErr: true,
		},
		{
			name:    "acknowledgement with all required fields",
			env:     Envelope{Type: TypeAcknowledgement, ProtocolVersion: CurrentProtocolVersion, MessageID: "msg-1", Payload: payload},
			wantErr: false,
		},
		{
			name:    "sync_request missing documentId",
			env:     Envelope{Type: TypeSyncRequest, ProtocolVersion: CurrentProtocolVersion, ClientID: "client-1", Payload: payload},
			wantErr: true,
		},
		{
			name:    "sync_response missing payload",
			env:     Envelope{Type: TypeSyncResponse, ProtocolVersion: CurrentProtocolVersion, DocumentID: "doc-1"},
			wantErr: true,
		},
		{
			name:    "presence_update missing clientId",
			env:     Envelope{Type: TypePresenceUpdate, ProtocolVersion: CurrentProtocolVersion, DocumentID: "doc-1", Payload: payload},
			wantErr: true,
		},
		{
			name:    "cursor_update with all required fields",
			env:     Envelope{Type: TypeCursorUpdate, ProtocolVersion: CurrentProtocolVersion, DocumentID: "doc-1", ClientID: "client-1", Payload: payload},
			wantErr: false,
		},
		{
			name:    "error missing payload",
			env:     Envelope{Type: TypeError, ProtocolVersion: CurrentProtocolVersion},
			wantErr: true,
		},
		{
			name:    "error with payload",
			env:     Envelope{Type: TypeError, ProtocolVersion: CurrentProtocolVersion, Payload: payload},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvelope(tt.env)
			if tt.wantErr && err == nil {
				t.Error("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
