package crdt

// Identifier is a globally-unique, deterministically-orderable identity
// used for both CRDT elements and operations.For more details see docs/crdt-specification.md.

type Identifier struct {
	ClientID uint64 `json:"clientId"`
	Counter  uint64 `json:"counter"`
}

// zeroClientID is reserved and never assigned to a real client.
const zeroClientID uint64 = 0

// RootID is used as the ParentElementID for elements inserted at the very start of the document.
var RootID = Identifier{ClientID: zeroClientID, Counter: 0}

func (id Identifier) IsRoot() bool {
	return id == RootID
}

// Less reports whether id sorts strictly before other.
func (id Identifier) Less(other Identifier) bool {
	if id.Counter != other.Counter {
		return id.Counter < other.Counter
	}
	return id.ClientID < other.ClientID
}

// Greater reports whether id sorts strictly after other.
func (id Identifier) Greater(other Identifier) bool {
	return other.Less(id)
}
