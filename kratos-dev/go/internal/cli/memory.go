package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/spf13/cobra"
)

// memoryTextMaxLen is the maximum allowed length for a single memory's text,
// counted in characters (a CJK fact is not penalised for its byte length).
const memoryTextMaxLen = 200

// MemoryCmd returns the 'memory' subcommand
func MemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage the persistent user memory model",
		Long:  "Add, list, and remove durable user facts (preferences, habits, weak spots) stored in SQLite",
	}

	cmd.AddCommand(MemoryAddCmd())
	cmd.AddCommand(MemoryListCmd())
	cmd.AddCommand(MemoryRemoveCmd())

	return cmd
}

// MemoryAddCmd adds a new user memory
func MemoryAddCmd() *cobra.Command {
	var category, project string
	var replaceID int64
	var force bool

	cmd := &cobra.Command{
		Use:   "add <text>",
		Short: "Add a new user memory",
		Long: `Add a durable user fact.

Near-duplicates are rejected: when an existing memory shares most of its words
with the new text the command exits non-zero and names that memory. Re-run with
--replace <id> to supersede it in place, or --force to keep both. Use --project
to scope a project-specific fact (it is then only injected in that project).`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			text := strings.TrimSpace(args[0])
			if text == "" {
				return fmt.Errorf("memory text is empty")
			}
			if n := utf8.RuneCountInString(text); n > memoryTextMaxLen {
				return fmt.Errorf("memory text exceeds %d characters (got %d) — shorten the fact instead of truncating it", memoryTextMaxLen, n)
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			proj := projectOrEmpty(project)

			if replaceID > 0 {
				memory, err := db.ReplaceMemory(conn, replaceID, text, category, proj)
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"status": "replaced",
					"memory": memory,
				})
			}

			if !force {
				dup, score, err := db.FindSimilarMemory(conn, text)
				if err != nil {
					return err
				}
				if dup != nil {
					return fmt.Errorf("near-duplicate of memory %d (similarity %.2f): %q — re-run with --replace %d to supersede it, or --force to keep both",
						dup.ID, score, dup.Text, dup.ID)
				}
			}

			memory, err := db.AddMemoryWithProject(conn, text, category, proj)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "added",
				"memory": memory,
			})
		},
	}

	cmd.Flags().StringVar(&category, "category", "context", "Category: preference, habit, weak-spot, context")
	cmd.Flags().StringVar(&project, "project", "", "Project root this fact belongs to (omit for a global fact)")
	cmd.Flags().Int64Var(&replaceID, "replace", 0, "Supersede memory <id> in place instead of adding a new row")
	cmd.Flags().BoolVar(&force, "force", false, "Add even when a near-duplicate exists")
	return cmd
}

// parseSince turns "7d", "36h", "30m" or a bare number of days into a duration.
func parseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, nil
	}
	unit := time.Duration(24) * time.Hour
	num := s
	switch s[len(s)-1] {
	case 'd':
		num = s[:len(s)-1]
	case 'h':
		unit = time.Hour
		num = s[:len(s)-1]
	case 'm':
		unit = time.Minute
		num = s[:len(s)-1]
	}
	n, err := strconv.ParseFloat(num, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid --since %q: use <n>d, <n>h, <n>m, or a number of days", s)
	}
	return time.Duration(n * float64(unit)), nil
}

// MemoryListCmd lists user memories
func MemoryListCmd() *cobra.Command {
	var category, project, since string
	var limit int
	var idsOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List user memories (newest first)",
		Long: `List user memories, newest first.

Without flags every row is returned. Callers that only need to dedupe or to
inject a few facts should pass --limit (and --project) so the output stays
small; "total" in the result reports how many rows matched before the limit.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			opts := db.MemoryListOpts{Category: category, Project: projectOrEmpty(project), Limit: limit}
			if since != "" {
				d, err := parseSince(since)
				if err != nil {
					return err
				}
				if d > 0 {
					opts.SinceMs = time.Now().Add(-d).UnixMilli()
				}
			}

			memories, err := db.ListMemoriesOpts(conn, opts)
			if err != nil {
				return err
			}
			total := len(memories)
			if limit > 0 {
				if total, err = db.CountMemories(conn, opts); err != nil {
					return err
				}
			}

			if idsOnly {
				ids := make([]int64, 0, len(memories))
				for _, m := range memories {
					ids = append(ids, m.ID)
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"ids":   ids,
					"count": len(ids),
					"total": total,
				})
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"category": category,
				"project":  opts.Project,
				"memories": memories,
				"count":    len(memories),
				"total":    total,
			})
		},
	}

	cmd.Flags().StringVar(&category, "category", "all", "Filter by category: preference, habit, weak-spot, context, all")
	cmd.Flags().StringVar(&project, "project", "", "Only memories scoped to this project root")
	cmd.Flags().StringVar(&since, "since", "", "Only memories newer than this age: 7d, 36h, 30m, or days as a number")
	cmd.Flags().IntVar(&limit, "limit", 0, "Return at most N memories (0 = all)")
	cmd.Flags().BoolVar(&idsOnly, "ids-only", false, "Return only ids (for dedupe checks)")
	return cmd
}

// MemoryRemoveCmd removes a user memory
func MemoryRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a user memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid ID: %s", args[0])
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			if err := db.RemoveMemory(conn, id); err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "removed",
				"id":     id,
			})
		},
	}
}
