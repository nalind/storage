package storage

import (
	"github.com/containers/storage/pkg/lockfile"
)

// A Locker represents a file lock where the file is used to cache an
// identifier of the last party that made changes to whatever's being protected
// by the lock.
type Locker interface {
	lockfile.Locker
}

// GetLockfile opens a read-write lock file, creating it if necessary.  The
// Locker object it returns will be returned unlocked.
func GetLockfile(path string) (Locker, error) {
	return lockfile.Get(path)
}

// GetROLockfile opens a read-only lock file.  The Locker object it returns
// will be returned unlocked.
func GetROLockfile(path string) (Locker, error) {
	return lockfile.GetRO(path)
}
