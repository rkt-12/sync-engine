package protocol

import "errors"

var (
	ErrUnsupportedProtocolVersion = errors.New("protocol: unsupported protocol version")

	ErrUnknownMessageType = errors.New("protocol: unknown message type")

	ErrMissingField = errors.New("protocol: missing required field")

	ErrMessageTooLarge = errors.New("protocol: message exceeds maximum size")
)
