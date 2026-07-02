package dotgit

import (
	"os"
	"path/filepath"
	"github.com/go-git/go-git/v5/plumbing"
)

// SetRef updates a reference, ensuring that the lock file is cleaned up on failure.
func (d *DotGit) SetRef(ref *plumbing.Reference) error {
	path := d.refPath(ref.Name())
	lockPath := path + ".lock"

	// Acquire lock
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			lock.Close()
			os.Remove(lockPath)
		}
	}()

	// Perform write operations
	_, err = lock.WriteString(ref.Hash().String() + "\n")
	if err != nil {
		return err
	}
	lock.Close()

	// Commit the lock
	err = os.Rename(lockPath, path)
	if err != nil {
		return err
	}

	committed = true
	return nil
}