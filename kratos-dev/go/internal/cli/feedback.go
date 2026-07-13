package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/spf13/cobra"
)

// feedbackLessonMaxLen is the maximum allowed length for a single lesson.
const feedbackLessonMaxLen = 200

// normalizeAgent lowercases an agent name and strips the "kratos:" prefix so
// "kratos:Ares" and "ares" address the same lesson bucket.
func normalizeAgent(agent string) string {
	return strings.TrimPrefix(strings.ToLower(agent), "kratos:")
}

// FeedbackCmd returns the 'feedback' subcommand
func FeedbackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Manage per-agent lessons from user corrections",
		Long:  "Add, list, and remove lessons a specific god-agent should apply next time, stored in SQLite",
	}

	cmd.AddCommand(FeedbackAddCmd())
	cmd.AddCommand(FeedbackListCmd())
	cmd.AddCommand(FeedbackRemoveCmd())

	return cmd
}

// FeedbackAddCmd adds a new agent feedback lesson
func FeedbackAddCmd() *cobra.Command {
	var agent string

	cmd := &cobra.Command{
		Use:   "add <lesson>",
		Short: "Add a lesson for a specific agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lesson := args[0]
			if len(lesson) > feedbackLessonMaxLen {
				return fmt.Errorf("lesson exceeds %d characters (got %d)", feedbackLessonMaxLen, len(lesson))
			}

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			feedback, err := db.AddFeedback(conn, normalizeAgent(agent), lesson, getProject())
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status":   "added",
				"feedback": feedback,
			})
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "God-agent the lesson applies to (e.g. ares, hermes)")
	_ = cmd.MarkFlagRequired("agent")
	return cmd
}

// FeedbackListCmd lists agent feedback lessons
func FeedbackListCmd() *cobra.Command {
	var agent string
	var limit int
	var preferProject bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agent lessons (all projects)",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.InitDB(conn); err != nil {
				return fmt.Errorf("failed to init db: %w", err)
			}

			project := ""
			if preferProject {
				project = getProject()
			}

			feedback, err := db.ListFeedback(conn, normalizeAgent(agent), project, limit)
			if err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"agent":    normalizeAgent(agent),
				"feedback": feedback,
				"count":    len(feedback),
			})
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "Filter by god-agent (default: all)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum lessons to return (0 = all)")
	cmd.Flags().BoolVar(&preferProject, "prefer-project", false, "Sort current-project lessons first")
	return cmd
}

// FeedbackRemoveCmd removes an agent feedback lesson
func FeedbackRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove an agent lesson",
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

			if err := db.RemoveFeedback(conn, id); err != nil {
				return err
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
				"status": "removed",
				"id":     id,
			})
		},
	}
}
