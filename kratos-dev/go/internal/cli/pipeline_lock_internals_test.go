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

	// The goroutine sends its error back instead of calling require itself:
	// require in a non-test goroutine only exits that goroutine, so the real
	// error got buried under a misleading "never completed" timeout.
	got := make(chan error, 1)
	go func() {
		r2, err := acquireStatusLock(path)
		if err == nil {
			r2()
		}
		got <- err
	}()

	select {
	case err := <-got:
		t.Fatalf("second acquire returned while first lock still held (err=%v)", err)
	case <-time.After(80 * time.Millisecond):
	}

	release()
	select {
	case err := <-got:
		require.NoError(t, err, "second acquire failed after release")
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

// TestAcquireStatusLockUndeletableStaleLockTimesOut checks that a stale lock
// another process still holds open does not spin forever. On Windows the
// open handle makes os.Remove fail, and acquire must fall through to its
// deadline instead of looping without a sleep or deadline check. On POSIX the
// remove succeeds and acquire gets the lock. Either way it must return.
func TestAcquireStatusLockUndeletableStaleLockTimesOut(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "status.json")
	lockPath := path + ".lock"
	require.NoError(t, os.WriteFile(lockPath, []byte("99999\n"), 0o644))
	stale := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(lockPath, stale, stale))

	holder, err := os.Open(lockPath)
	require.NoError(t, err)
	defer holder.Close()

	type result struct {
		release func()
		err     error
	}
	done := make(chan result, 1)
	go func() {
		r, err := acquireStatusLock(path)
		done <- result{r, err}
	}()

	select {
	case res := <-done:
		if res.err == nil {
			res.release()
		}
	case <-time.After(statusLockTimeout + 3*time.Second):
		t.Fatal("acquireStatusLock spun past its timeout on an undeletable stale lock")
	}
}
