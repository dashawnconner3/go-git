package dotgit

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// globalMutex protects against concurrent SetRef calls across goroutines.
var globalMutex sync.Mutex

// SetRef updates a reference with safe lock file handling.
// Guarantees:
//   - defer-based cleanup on any failure or panic
//   - thread-safe via global mutex
//   - stale lock detection (cleanup locks older than 2s from crashed processes)
//   - atomic rename after successful write
func (d *DotGit) SetRef(ref *plumbing.Reference) error {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	path := d.refPath(ref.Name())
	lockPath := path + ".lock"

	// Clean up stale locks from crashed processes
	if err := cleanStaleLock(lockPath); err != nil {
		return fmt.Errorf("checking stale lock %s: %w", lockPath, err)
	}

	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("acquiring lock %s: %w", lockPath, err)
	}

	committed := false
	defer func() {
		// Ensure file handle is closed before any cleanup
		lock.Close()
		if !committed {
			// Best-effort removal; stale lock cleaner handles leftovers
			_ = os.Remove(lockPath)
		}
	}()

	// Write reference data
	if _, err = lock.WriteString(ref.Hash().String() + "\n"); err != nil {
		return fmt.Errorf("writing ref %s: %w", ref.Name(), err)
	}

	// Sync to disk before rename to avoid partial writes
	if err = lock.Sync(); err != nil {
		return fmt.Errorf("syncing lock file %s: %w", lockPath, err)
	}

	// Close before rename (required on Windows)
	if err = lock.Close(); err != nil {
		return fmt.Errorf("closing lock file %s: %w", lockPath, err)
	}

	// Atomic commit: rename lock to final path
	if err = os.Rename(lockPath, path); err != nil {
		return fmt.Errorf("committing ref %s: %w", ref.Name(), err)
	}

	committed = true
	return nil
}

// cleanStaleLock removes a lock file if it appears to be from a crashed process.
// A lock is considered stale if it was created more than 2 seconds ago.
func cleanStaleLock(lockPath string) error {
	info, err := os.Stat(lockPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// Lock file older than 2 seconds is stale from a crashed process
	if time.Since(info.ModTime()) > 2*time.Second {
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

// refPath returns the filesystem path for a reference name.
func (d *DotGit) refPath(name plumbing.ReferenceName) string {
	return filepath.Join(string(d.dir), name.String())
}
