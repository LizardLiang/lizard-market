package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/spf13/cobra"
)

// sessionIdleAbandonAfter is how long a zero-step 'active' row may sit before
// `session start` closes it as a ghost. SessionStart fires with a throwaway
// id before a resume/fork, and that id never receives a step or a SessionEnd.
const sessionIdleAbandonAfter = 24 * time.Hour

// SessionGcCmd returns `session gc`: close ghost 'active' rows and prune old
// per-session ledger files under ~/.kratos/sessions.
func SessionGcCmd() *cobra.Command {
	var days int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Mark ghost 'active' sessions abandoned and prune old ledger files",
		Long: `Close session rows that will never be ended by a hook.

Two rules: an 'active' row with zero steps older than 24 hours (the throwaway id
Claude Code's SessionStart fires before a resume/fork), and any 'active' row older
than --days (a window closed without SessionEnd). Matching rows become 'abandoned';
a later step or SessionEnd against them still works. Ledger files under
~/.kratos/sessions older than --days are deleted. --dry-run only counts.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if days <= 0 {
				return fmt.Errorf("--days must be positive (got %d)", days)
			}
			stale := time.Duration(days) * 24 * time.Hour

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()
			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			candidates, err := db.CountStaleSessions(conn, sessionIdleAbandonAfter, stale)
			if err != nil {
				return err
			}
			var abandoned int64
			if !dryRun {
				if abandoned, err = db.AbandonStaleSessions(conn, sessionIdleAbandonAfter, stale); err != nil {
					return err
				}
			}
			ledgerCandidates, ledgerRemoved, err := pruneSessionLedger(stale, dryRun)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"dry_run":            dryRun,
				"days":               days,
				"session_candidates": candidates,
				"sessions_abandoned": abandoned,
				"ledger_candidates":  ledgerCandidates,
				"ledger_removed":     ledgerRemoved,
			})
		},
	}

	cmd.Flags().IntVar(&days, "days", 7, "Abandon any active session, and delete any ledger file, older than this many days")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report what would change without writing")
	return cmd
}

// sessionLedgerDir is ~/.kratos/sessions, where session-start.cjs writes one
// <session_id>.json per Claude Code session; "" when the home dir is unknown.
func sessionLedgerDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".kratos", "sessions")
}

// pruneSessionLedger deletes ledger files older than maxAge and returns
// (candidates, removed). A missing directory is not an error.
func pruneSessionLedger(maxAge time.Duration, dryRun bool) (int, int, error) {
	dir := sessionLedgerDir()
	if dir == "" {
		return 0, 0, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("failed to read %s: %w", dir, err)
	}
	cutoff := time.Now().Add(-maxAge)
	candidates, removed := 0, 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		candidates++
		if dryRun {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			removed++
		}
	}
	return candidates, removed, nil
}
