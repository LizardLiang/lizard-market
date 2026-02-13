package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/db"
)

// StepCmd returns the 'step' command
func StepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "step",
		Short: "Manage session steps",
		Long:  "Record and query steps within sessions",
	}

	cmd.AddCommand(StepRecordAgentCmd())
	cmd.AddCommand(StepRecordFileCmd())
	cmd.AddCommand(StepListCmd())

	return cmd
}

// StepRecordAgentCmd records an agent spawn
func StepRecordAgentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "record-agent <session_id> <agent_name> <agent_model> <action>",
		Short: "Record an agent spawn step",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			agentName := args[1]
			agentModel := args[2]
			action := args[3]

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.RecordAgentSpawn(conn, sessionID, agentName, agentModel, action); err != nil {
				return fmt.Errorf("failed to record agent spawn: %w", err)
			}

			// Get updated step count
			session, err := db.GetSession(conn, sessionID)
			if err != nil {
				return err
			}

			result := map[string]interface{}{
				"status":      "success",
				"step_number": session.TotalSteps,
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
}

// StepRecordFileCmd records a file change
func StepRecordFileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "record-file <session_id> <action> <file_path>",
		Short: "Record a file modification step",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			action := args[1]
			filePath := args[2]

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := db.RecordFileChange(conn, sessionID, action, filePath); err != nil {
				return fmt.Errorf("failed to record file change: %w", err)
			}

			result := map[string]interface{}{
				"status": "success",
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
}

// StepListCmd lists all steps for a session
func StepListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <session_id>",
		Short: "List all steps for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			conn, err := db.GetConnection()
			if err != nil {
				return err
			}
			defer conn.Close()

			steps, err := db.GetStepsForSession(conn, sessionID)
			if err != nil {
				return fmt.Errorf("failed to get steps: %w", err)
			}

			result := map[string]interface{}{
				"session_id": sessionID,
				"steps":      steps,
				"count":      len(steps),
			}

			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
}
