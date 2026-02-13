package formatter

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// FormatSession formats a session as human-readable text
func FormatSession(session *models.Session) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Session: %s\n", session.SessionID))
	sb.WriteString(fmt.Sprintf("  Project: %s\n", session.Project))

	if session.FeatureName != nil {
		sb.WriteString(fmt.Sprintf("  Feature: %s\n", *session.FeatureName))
	}

	sb.WriteString(fmt.Sprintf("  Status: %s\n", FormatStatus(session.Status)))
	sb.WriteString(fmt.Sprintf("  Started: %s\n", FormatTimestamp(session.StartedAt)))

	if session.EndedAt != nil {
		sb.WriteString(fmt.Sprintf("  Ended: %s\n", FormatTimestamp(*session.EndedAt)))
		duration := FormatDuration(session.StartedAt, *session.EndedAt)
		sb.WriteString(fmt.Sprintf("  Duration: %s\n", duration))
	}

	if session.Summary != nil {
		sb.WriteString(fmt.Sprintf("  Summary: %s\n", *session.Summary))
	}

	sb.WriteString(fmt.Sprintf("  Steps: %d\n", session.TotalSteps))
	sb.WriteString(fmt.Sprintf("  Agents: %d\n", session.TotalAgentsSpawned))

	return sb.String()
}

// FormatSessionContext formats a full session context
func FormatSessionContext(context *models.SessionContext) string {
	var sb strings.Builder

	// Format session
	sb.WriteString("Session:\n")
	sb.WriteString("========\n")
	sb.WriteString(FormatSession(context.Session))
	sb.WriteString("\n")

	// Format steps
	if len(context.Steps) > 0 {
		sb.WriteString("Steps:\n")
		sb.WriteString("======\n")
		for i, step := range context.Steps {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, step.StepType, step.Action))

			if step.AgentName != nil {
				sb.WriteString(fmt.Sprintf(" (agent: %s", *step.AgentName))
				if step.AgentModel != nil {
					sb.WriteString(fmt.Sprintf("/%s", *step.AgentModel))
				}
				sb.WriteString(")")
			}

			if step.Target != nil {
				sb.WriteString(fmt.Sprintf(" → %s", *step.Target))
			}

			if step.Result != nil {
				sb.WriteString(fmt.Sprintf(" [%s]", *step.Result))
			}

			sb.WriteString("\n")

			if step.Context != nil && *step.Context != "" {
				sb.WriteString(fmt.Sprintf("   %s\n", *step.Context))
			}
		}
	} else {
		sb.WriteString("No steps recorded\n")
	}

	return sb.String()
}

// FormatSessionList formats a list of sessions
func FormatSessionList(sessions []*models.Session) string {
	if len(sessions) == 0 {
		return "No sessions found\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d session(s):\n\n", len(sessions)))

	for i, session := range sessions {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, session.SessionID))
		sb.WriteString(fmt.Sprintf("   Project: %s\n", session.Project))

		if session.FeatureName != nil {
			sb.WriteString(fmt.Sprintf("   Feature: %s\n", *session.FeatureName))
		}

		sb.WriteString(fmt.Sprintf("   Status: %s\n", FormatStatus(session.Status)))
		sb.WriteString(fmt.Sprintf("   Started: %s\n", FormatTimestamp(session.StartedAt)))

		if session.Summary != nil {
			sb.WriteString(fmt.Sprintf("   Summary: %s\n", *session.Summary))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatTimestamp formats a Unix timestamp in milliseconds to human-readable format
func FormatTimestamp(timestampMs int64) string {
	ts := time.UnixMilli(timestampMs)
	now := time.Now()
	duration := now.Sub(ts)

	// Relative time for recent events
	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}

	// Absolute date for older events
	return ts.Format("2006-01-02 15:04")
}

// FormatDuration formats a duration between two timestamps
func FormatDuration(startMs, endMs int64) string {
	duration := time.Duration(endMs-startMs) * time.Millisecond

	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		days := int(duration.Hours() / 24)
		hours := int(duration.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}

// FormatStatus formats a session status with appropriate styling
func FormatStatus(status string) string {
	// In a terminal, this could use ANSI colors
	// For now, just return the status as-is
	switch status {
	case "active":
		return "🟢 active"
	case "completed":
		return "✅ completed"
	case "abandoned":
		return "⚠️  abandoned"
	default:
		return status
	}
}
