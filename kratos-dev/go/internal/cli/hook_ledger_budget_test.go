package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
)

// TestRecordPromptLedgerBoundedByLockedDB pins promptLedgerBudget: with
// another writer holding the database lock, recordPromptLedger must return
// inside the budget instead of waiting out busy_timeout (5000ms), which alone
// equals the UserPromptSubmit hook timeout and got the routing output
// discarded ("timed out after 5s — output discarded").
func TestRecordPromptLedgerBoundedByLockedDB(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	t.Setenv("KRATOS_MEMORY_DB", filepath.Join(home, "memory.db"))

	holder, err := db.GetConnection()
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if err := db.InitDB(holder); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	lock, err := holder.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}

	raw := []byte(`{"session_id":"` + testLedgerSession + `","cwd":"C:/repo","prompt":"/kratos:iris fix the build"}`)
	start := time.Now()
	recordPromptLedger(raw)
	elapsed := time.Since(start)

	if elapsed > promptLedgerBudget+time.Second {
		t.Errorf("recordPromptLedger took %v under a held lock, want <= %v", elapsed, promptLedgerBudget+time.Second)
	}
	// The file ledger is written before the database half and must not wait on it.
	if got := ledgerString(mustReadLedger(t), ledgerKeyInlineGod); got != "iris" {
		t.Errorf("inline_god = %q, want iris", got)
	}

	// Release the lock and let the abandoned write finish, so the temp dir
	// can be removed on Windows.
	if _, err := lock.ExecContext(ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	lock.Close()
	deadline := time.Now().Add(6 * time.Second)
	for {
		n, err := db.GetSessionCount(holder)
		if err == nil && n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("abandoned session write never completed (count=%d, err=%v)", n, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
