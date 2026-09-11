package engine

import (
	"errors"
)

// batchOp represents a staged mutation within a WriteBatch.
type batchOp struct {
	op    OpType
	key   string
	value []byte
}

// WriteBatch stages multiple operations to be committed atomically to disk and memory.
type WriteBatch struct {
	ops []batchOp
}

// NewWriteBatch creates a new empty WriteBatch.
func NewWriteBatch() *WriteBatch {
	return &WriteBatch{}
}

// Set stages an insert or update in the batch.
func (b *WriteBatch) Set(key string, value []byte) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}
	b.ops = append(b.ops, batchOp{op: OpSet, key: key, value: value})
	return nil
}

// Delete stages a deletion in the batch.
func (b *WriteBatch) Delete(key string) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}
	b.ops = append(b.ops, batchOp{op: OpDelete, key: key})
	return nil
}

// Clear stages a clear in the batch.
func (b *WriteBatch) Clear() {
	b.ops = append(b.ops, batchOp{op: OpClear})
}

// Len returns the count of staged operations in the batch.
func (b *WriteBatch) Len() int {
	return len(b.ops)
}

// Reset clears all staged operations from the batch, allowing it to be reused.
func (b *WriteBatch) Reset() {
	b.ops = b.ops[:0]
}
