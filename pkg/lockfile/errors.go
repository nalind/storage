package lockfile

import (
	"errors"
)

var (
	// ErrLockReadOnly indicates that the caller only took a read-only lock, and is not allowed to write.
	ErrLockReadOnly = errors.New("lock is not a read-write lock")
)
