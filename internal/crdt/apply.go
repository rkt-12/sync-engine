package crdt

import "fmt"

// this applies a single operation to the document.
func (d *Document) Apply(op Operation) error {
	switch o := op.(type) {
	case InsertOperation:
		return d.Insert(o)
	case DeleteOperation:
		return d.Delete(o)
	default:
		return fmt.Errorf("%w: %T", ErrUnsupportedOperation, op)
	}
}

// ApplyBatch applies a sequence of operations in the given order. ApplyBatch stops at the first
// error rather than silently skipping the remainder.
func (d *Document) ApplyBatch(ops []Operation) error {
	for i, op := range ops {
		if err := d.Apply(op); err != nil {
			return fmt.Errorf("applying operation %d of %d: %w", i+1, len(ops), err)
		}
	}
	return nil
}
