package protocol

import "fmt"

// MaxMessageSize is the maximum permitted size, in bytes
const MaxMessageSize = 64 * 1024 // 64 KiB

// ValidateEnvelope checks that env's ProtocolVersion is supported, its
// Type is known, and every field required for that Type is present.
func ValidateEnvelope(env Envelope) error {
	if env.ProtocolVersion != CurrentProtocolVersion {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedProtocolVersion, env.ProtocolVersion, CurrentProtocolVersion)
	}

	switch env.Type {
	case TypeJoinDocument:
		return requireFields(env, "documentId", "clientId")

	case TypeInitialSync:
		return requireFields(env, "documentId", "payload")

	case TypeOperation, TypeOperationBatch:
		return requireFields(env, "documentId", "clientId", "messageId", "payload")

	case TypeAcknowledgement:
		return requireFields(env, "messageId", "payload")

	case TypeSyncRequest:
		return requireFields(env, "documentId", "clientId", "payload")

	case TypeSyncResponse:
		return requireFields(env, "documentId", "payload")

	case TypePresenceUpdate, TypeCursorUpdate:
		return requireFields(env, "documentId", "clientId", "payload")

	case TypeError:
		return requireFields(env, "payload")

	case TypePing, TypePong:
		// No fields required beyond Type and ProtocolVersion.
		return nil

	default:
		return fmt.Errorf("%w: %q", ErrUnknownMessageType, env.Type)
	}
}

// requireFields checks that each named field is non-empty on env.
func requireFields(env Envelope, fields ...string) error {
	for _, field := range fields {
		var present bool
		switch field {
		case "documentId":
			present = env.DocumentID != ""
		case "clientId":
			present = env.ClientID != ""
		case "messageId":
			present = env.MessageID != ""
		case "payload":
			present = len(env.Payload) > 0
		default:
			// Programmer error in this package
			panic(fmt.Sprintf("protocol: requireFields: unknown field name %q", field))
		}
		if !present {
			return fmt.Errorf("%w: %q is required for message type %q", ErrMissingField, field, env.Type)
		}
	}
	return nil
}
