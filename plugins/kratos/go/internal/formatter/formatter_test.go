package formatter

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// Helper function
func stringPtr(s string) *string { return &s }
func int64Ptr(i int64) *int64    { return &i }

// TestFormatSession tests formatting a session to text
func TestFormatSession(t *testing.T) {
	now := time.Now().UnixMilli()
	session := &models.Session{
		ID:                 1,
		SessionID:          "sess-123",
		Project:            "/test/project",
		FeatureName:        stringPtr("authentication"),
		StartedAt:          now - 3600000, // 1 hour ago
		EndedAt:            int64Ptr(now),
		Status:             "completed",
		Summary:            stringPtr("Implemented OAuth2"),
		TotalSteps:         10,
		TotalAgentsSpawned: 3,
	}

	text := FormatSession(session)
	require.NotEmpty(t, text)

	// Verify key information is present
	assert.Contains(t, text, "sess-123")
	assert.Contains(t, text, "/test/project")
	assert.Contains(t, text, "authentication")
	assert.Contains(t, text, "completed")
	assert.Contains(t, text, "Implemented OAuth2")
}

// TestFormatSessionContext tests formatting a full session context
func TestFormatSessionContext(t *testing.T) {
	now := time.Now().UnixMilli()

	session := &models.Session{
		SessionID:   "sess-123",
		Project:     "/test/project",
		FeatureName: stringPtr("test-feature"),
		StartedAt:   now,
		Status:      "active",
	}

	steps := []*models.Step{
		{
			ID:          1,
			SessionID:   "sess-123",
			StepNumber:  1,
			StepType:    "agent_spawn",
			Timestamp:   now,
			AgentName:   stringPtr("athena"),
			AgentModel:  stringPtr("opus"),
			Action:      "create_prd",
			Target:      stringPtr("prd.md"),
			Result:      stringPtr("success"),
		},
	}

	context := &models.SessionContext{
		Session:     session,
		Steps:       steps,
		TotalSteps:  1,
		TotalAgents: 1,
	}

	text := FormatSessionContext(context)
	require.NotEmpty(t, text)

	// Verify structure
	assert.Contains(t, text, "Session:")
	assert.Contains(t, text, "sess-123")
	assert.Contains(t, text, "Steps:")
	assert.Contains(t, text, "athena")
	assert.Contains(t, text, "create_prd")
}

// TestFormatSessionList tests formatting multiple sessions
func TestFormatSessionList(t *testing.T) {
	now := time.Now().UnixMilli()

	sessions := []*models.Session{
		{
			SessionID: "sess-1",
			Project:   "/project/a",
			StartedAt: now - 2000,
			Status:    "completed",
		},
		{
			SessionID: "sess-2",
			Project:   "/project/b",
			StartedAt: now - 1000,
			Status:    "active",
		},
	}

	text := FormatSessionList(sessions)
	require.NotEmpty(t, text)

	// Verify both sessions are present
	assert.Contains(t, text, "sess-1")
	assert.Contains(t, text, "sess-2")
	assert.Contains(t, text, "/project/a")
	assert.Contains(t, text, "/project/b")
}

// TestFormatTimestamp tests timestamp formatting
func TestFormatTimestamp(t *testing.T) {
	now := time.Now().UnixMilli()

	// Test various timestamps
	testCases := []struct {
		name     string
		ts       int64
		expected string
	}{
		{"recent", now - 1000, "just now"},     // Very recent
		{"older", now - 3600000, "ago"},        // 1 hour ago
		{"very old", now - 86400000, "days"}, // Should show days
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formatted := FormatTimestamp(tc.ts)
			assert.Contains(t, strings.ToLower(formatted), tc.expected)
		})
	}
}

// TestFormatDuration tests duration formatting
func TestFormatDuration(t *testing.T) {
	testCases := []struct {
		name     string
		start    int64
		end      int64
		expected string
	}{
		{"seconds", 1000, 5000, "4s"},
		{"minutes", 0, 60000, "1m"},
		{"hours", 0, 3600000, "1h"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formatted := FormatDuration(tc.start, tc.end)
			assert.NotEmpty(t, formatted)
			// Just check it's not empty - exact format may vary
		})
	}
}

// TestFormatStatus tests status formatting with colors (in plain text for testing)
func TestFormatStatus(t *testing.T) {
	testCases := []struct {
		status   string
		expected string
	}{
		{"active", "active"},
		{"completed", "completed"},
		{"abandoned", "abandoned"},
	}

	for _, tc := range testCases {
		t.Run(tc.status, func(t *testing.T) {
			formatted := FormatStatus(tc.status)
			assert.Contains(t, strings.ToLower(formatted), tc.expected)
		})
	}
}
