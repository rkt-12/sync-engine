package store

import (
	"encoding/binary"
	"fmt"

	"sync-engine/internal/crdt"
)

// identifierEncodedLen is the fixed size, in bytes, of an encoded crdt.
// Identifier: 8 bytes ClientID (big-endian) + 8 bytes Counter (big-endian).
const identifierEncodedLen = 16

// encodeIdentifier serializes a crdt.
func encodeIdentifier(id crdt.Identifier) []byte {
	buf := make([]byte, identifierEncodedLen)
	binary.BigEndian.PutUint64(buf[0:8], id.ClientID)
	binary.BigEndian.PutUint64(buf[8:16], id.Counter)
	return buf
}

// decodeIdentifier is the inverse of encodeIdentifier.
func decodeIdentifier(b []byte) (crdt.Identifier, error) {
	if len(b) != identifierEncodedLen {
		return crdt.Identifier{}, fmt.Errorf("store: invalid identifier encoding: got %d bytes, want %d", len(b), identifierEncodedLen)
	}
	return crdt.Identifier{
		ClientID: binary.BigEndian.Uint64(b[0:8]),
		Counter:  binary.BigEndian.Uint64(b[8:16]),
	}, nil
}
