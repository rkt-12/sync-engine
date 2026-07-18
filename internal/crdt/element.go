package crdt

// element is an internal node representing one character in the document
type element struct {
	id      ElementID
	parent  ElementID
	value   rune
	deleted bool // tombstone flag, the element is never physically removed
}

func (id ElementID) IsRoot() bool {
	return Identifier(id).IsRoot()
}
