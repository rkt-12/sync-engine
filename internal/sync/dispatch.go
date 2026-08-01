package sync

import (
	"context"
	"encoding/json"
	"fmt"

	"sync-engine/internal/crdt"
	"sync-engine/internal/protocol"
)

// HandleEnvelope routes one decoded, already-validated envelope from
// `from` to the appropriate DocumentRoom method, based on env.Type.
func HandleEnvelope(ctx context.Context, room *DocumentRoom, from *Client, env protocol.Envelope) {
	var err error

	switch env.Type {
	case protocol.TypeOperation:
		var op crdt.Operation
		if op, err = protocol.DecodeOperation(env); err == nil {
			err = room.HandleOperation(ctx, from, op, env.MessageID)
		}

	case protocol.TypeOperationBatch:
		var ops []crdt.Operation
		if ops, err = protocol.DecodeOperationBatch(env); err == nil {
			err = room.HandleOperationBatch(ctx, from, ops, env.MessageID)
		}

	case protocol.TypeSyncRequest:
		var payload protocol.SyncRequestPayload
		if err = json.Unmarshal(env.Payload, &payload); err == nil {
			err = room.HandleSyncRequest(ctx, from, payload.LastKnownServerSequence)
		}

	case protocol.TypePresenceUpdate, protocol.TypeCursorUpdate:
		room.BroadcastEphemeral(from, env)

	case protocol.TypePing:
		from.Enqueue(protocol.Envelope{
			Type:            protocol.TypePong,
			ProtocolVersion: protocol.CurrentProtocolVersion,
		})

	case protocol.TypeJoinDocument:

	default:
		err = fmt.Errorf("sync: no handler for message type %q", env.Type)
	}

	if err != nil {
		if errEnv, buildErr := buildErrorEnvelope(err); buildErr == nil {
			from.Enqueue(errEnv)
		}
	}
}

// buildErrorEnvelope wraps cause into a TypeError envelope.
func buildErrorEnvelope(cause error) (protocol.Envelope, error) {
	payload, err := json.Marshal(protocol.ErrorPayload{
		Code:    "processing_error",
		Message: cause.Error(),
	})
	if err != nil {
		return protocol.Envelope{}, err
	}
	return protocol.Envelope{
		Type:            protocol.TypeError,
		ProtocolVersion: protocol.CurrentProtocolVersion,
		Payload:         payload,
	}, nil
}
