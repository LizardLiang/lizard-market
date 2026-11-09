package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestAcquireStatusLockSerializes checks the lock helper itself: a second
// acquire must block until the first is released.
func TestAcquireStatusLockSerializes(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "status.json")

	release, err := acquireStatusLock(path)
	require.NoError(t, err)

	got := make(chan struct{})
	go func() {
		r2, err := acquireStatusLock(path)
		require.NoError(t, err)
		close(got)
		r2()
	}()

	select {
	case <-got:
		t.Fatal("second acquire succeeded while first lock still held")
	case <-time.After(80 * time.Millisecond):
	}

	release()
	select {
	case <-got:
	case <-time.After(statusLockTimeout):
		t.Fatal("second acquire never completed after release")
	}
}

// TestAcquireStatusLockReclaimsStaleLock checks that a lock file left behind
// by a crashed holder (older than statusLockStaleAge) does not wedge every
// future update.
func TestAcquireStatusLockReclaimsStaleLock(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "status.json")
	lockPath := path + ".lock"
	require.NoError(t, os.WriteFile(lockPath, []byte("99999\n"), 0o644))
	stale := time.Now().Add(-(statusLockStaleAge + time.Second))
	require.NoError(t, os.Chtimes(lockPath, stale, stale))

	release, err := acquireStatusLock(path)
	require.NoError(t, err)
	defer release()
}
