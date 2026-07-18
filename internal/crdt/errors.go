package crdt

import "errors"

var (
	ErrDocumentMismatch = errors.New("crdt: operation belongs to a different document")

	ErrDuplicateElement = errors.New("crdt: element already exists")

	ErrParentNotFound = errors.New("crdt: parent element not found")

	ErrUnsupportedOperation = errors.New("crdt: unsupported operation type")
)
