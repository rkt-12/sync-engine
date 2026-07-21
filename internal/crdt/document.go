package crdt

import (
	"fmt"
	"strings"
)

// Document is a single CRDT-managed sequence, one collaboratively edited text document.
type Document struct {
	id string

	elements map[ElementID]*element    // every element ever created, including tombstones
	children map[ElementID][]ElementID // parent -> direct children, kept sorted descending by Identifier

	appliedOps        map[OperationID]struct{} // dedup: operations already applied
	pendingTombstones map[ElementID]struct{}   // deletes whose target hasn't been inserted yet
}

func NewDocument(id string) *Document {
	return &Document{
		id:                id,
		elements:          make(map[ElementID]*element),
		children:          make(map[ElementID][]ElementID),
		appliedOps:        make(map[OperationID]struct{}),
		pendingTombstones: make(map[ElementID]struct{}),
	}
}

func (d *Document) ID() string { return d.id }

// HasOperation reports whether an operation with this ID has already been applied to this document.
func (d *Document) HasOperation(id OperationID) bool {
	_, ok := d.appliedOps[id]
	return ok
}

// Insert applies an InsertOperation, adding one new character to the document positioned relative to op.ParentElementID.
// It is idempotent: if op.OperationID has already been applied, Insert returns nil without modifying the document.
func (d *Document) Insert(op InsertOperation) error {
	if op.DocumentID != d.id {
		return fmt.Errorf("%w: operation is for document %q, this document is %q",
			ErrDocumentMismatch, op.DocumentID, d.id)
	}

	if d.HasOperation(OperationID(op.OperationID)) {
		return nil
	}

	if _, exists := d.elements[op.ElementID]; exists {
		return fmt.Errorf("%w: %+v", ErrDuplicateElement, op.ElementID)
	}

	if !op.ParentElementID.IsRoot() {
		if _, exists := d.elements[op.ParentElementID]; !exists {
			return fmt.Errorf("%w: %+v", ErrParentNotFound, op.ParentElementID)
		}
	}

	el := &element{
		id:     op.ElementID,
		parent: op.ParentElementID,
		value:  op.Value,
	}
	d.elements[op.ElementID] = el
	d.insertSorted(op.ParentElementID, op.ElementID)

	// Delete-before-insert: a delete targeting this element may have already arrived and been recorded as pending.
	if _, wasPending := d.pendingTombstones[op.ElementID]; wasPending {
		el.deleted = true
		delete(d.pendingTombstones, op.ElementID)
	}

	d.appliedOps[OperationID(op.OperationID)] = struct{}{}
	return nil
}

// Delete applies a DeleteOperation, tombstoning the element referenced by op.TargetElementID.
// If the target element has not been inserted on this replica yet, the
// deletion is recorded as pending and applied automatically the moment
// the corresponding Insert arrives.
func (d *Document) Delete(op DeleteOperation) error {
	if op.DocumentID != d.id {
		return fmt.Errorf("%w: operation is for document %q, this document is %q",
			ErrDocumentMismatch, op.DocumentID, d.id)
	}

	if d.HasOperation(OperationID(op.OperationID)) {
		return nil
	}

	if el, exists := d.elements[op.TargetElementID]; exists {
		el.deleted = true
	} else {
		d.pendingTombstones[op.TargetElementID] = struct{}{}
	}

	d.appliedOps[OperationID(op.OperationID)] = struct{}{}
	return nil
}

// Materialize returns the visible document text: every non-tombstoned element's value, concatenated in document order.
func (d *Document) Materialize() string {
	var b strings.Builder
	d.walk(ElementID(RootID), func(_ ElementID, el *element) {
		if !el.deleted {
			b.WriteRune(el.value)
		}
	})
	return b.String()
}

// VisibleSequence returns the ElementIDs of all non-tombstoned elements, in document order.
func (d *Document) VisibleSequence() []ElementID {
	var out []ElementID
	d.walk(ElementID(RootID), func(id ElementID, el *element) {
		if !el.deleted {
			out = append(out, id)
		}
	})
	return out
}

// walk performs a depth-first traversal of the tree rooted at parentID, in document order
func (d *Document) walk(parentID ElementID, visit func(ElementID, *element)) {
	for _, childID := range d.children[parentID] {
		el := d.elements[childID]
		visit(childID, el)
		d.walk(childID, visit)
	}
}

// insertSorted inserts childID into parentID's children list, maintaining
// descending Identifier order, per the RGA sibling-ordering rule
func (d *Document) insertSorted(parentID, childID ElementID) {
	siblings := d.children[parentID]

	idx := 0
	for idx < len(siblings) && Identifier(siblings[idx]).Greater(Identifier(childID)) {
		idx++
	}
	siblings = append(siblings, ElementID{})
	copy(siblings[idx+1:], siblings[idx:])
	siblings[idx] = childID
	d.children[parentID] = siblings
}
