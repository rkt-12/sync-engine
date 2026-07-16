package crdt

// ElementID identifies a CRDT element (one character in the document).
type ElementID Identifier

// OperationID identifies an operation.
type OperationID Identifier

// OperationKind discriminates the two operation types on the wire and in code that must branch on kind.
type OperationKind string

const (
	KindInsert OperationKind = "insert"
	KindDelete OperationKind = "delete"
)

// Operation is the common interface satisfied by InsertOperation and DeleteOperation.
type Operation interface {
	ID() OperationID
	Document() string
	Kind() OperationKind
}

// InsertOperation introduces one new character into the document, positioned immediately after ParentElementID.
type InsertOperation struct {
	OperationID     OperationID
	DocumentID      string
	ClientID        uint64
	LogicalClock    uint64
	ParentElementID ElementID
	ElementID       ElementID
	Value           rune
}

func (op InsertOperation) ID() OperationID     { return op.OperationID }
func (op InsertOperation) Document() string    { return op.DocumentID }
func (op InsertOperation) Kind() OperationKind { return KindInsert }

// DeleteOperation tombstones an existing element. The element itself is
// never removed from the structure only its visibility is flagged.
type DeleteOperation struct {
	OperationID     OperationID
	DocumentID      string
	ClientID        uint64
	LogicalClock    uint64
	TargetElementID ElementID
}

func (op DeleteOperation) ID() OperationID     { return op.OperationID }
func (op DeleteOperation) Document() string    { return op.DocumentID }
func (op DeleteOperation) Kind() OperationKind { return KindDelete }
