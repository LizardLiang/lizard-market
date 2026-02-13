package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// TestGetLastSessionForProject tests getting the most recent session for a project
func TestGetLastSessionForProject(t *testing.T) {
	db := NewTestDBWithSchema(t)

	now := time.Now().UnixMilli()
	project := "/test/project"

	// Create 3 sessions for the project
	sessions := []*models.Session{
		{
			SessionID: "old-session",
			Project:   project,
			StartedAt: now - 3000,
			EndedAt:   int64Ptr(now - 2500),
			Status:    "completed",
			Summary:   stringPtr("Old work"),
		},
		{
			SessionID: "middle-session",
			Project:   project,
			StartedAt: now - 2000,
			EndedAt:   int64Ptr(now - 1500),
			Status:    "completed",
			Summary:   stringPtr("Middle work"),
		},
		{
			SessionID: "recent-session",
			Project:   project,
			StartedAt: now - 1000,
			EndedAt:   int64Ptr(now - 500),
			Status:    "completed",
			Summary:   stringPtr("Recent work"),
		},
	}

	for _, s := range sessions {
		err := CreateSession(db, s)
		require.NoError(t, err)
	}

	// Test: Get last session
	lastSession, err := GetLastSessionForProject(db, project)
	require.NoError(t, err)
	require.NotNil(t, lastSession)
	assert.Equal(t, "recent-session", lastSession.SessionID)
	assert.Equal(t, "Recent work", *lastSession.Summary)
}

// TestGetLastSessionForProject_NoSessions tests when no sessions exist
func TestGetLastSessionForProject_NoSessions(t *testing.T) {
	db := NewTestDBWithSchema(t)

	session, err := GetLastSessionForProject(db, "/nonexistent/project")
	require.NoError(t, err)
	assert.Nil(t, session) // Should return nil, not error
}

// TestGetIncompleteFeatures tests getting features that aren't finished
func TestGetIncompleteFeatures(t *testing.T) {
	db := NewTestDBWithSchema(t)

	now := time.Now().UnixMilli()
	project := "/test/project"

	// Create sessions with and without feature names
	sessions := []*models.Session{
		{
			SessionID:   "no-feature",
			Project:     project,
			StartedAt:   now - 4000,
			EndedAt:     int64Ptr(now - 3500),
			Status:      "completed",
			FeatureName: nil, // No feature
		},
		{
			SessionID:   "incomplete-auth",
			Project:     project,
			FeatureName: stringPtr("authentication"),
			StartedAt:   now - 3000,
			EndedAt:     nil, // Still active
			Status:      "active",
		},
		{
			SessionID:   "completed-payment",
			Project:     project,
			FeatureName: stringPtr("payment"),
			StartedAt:   now - 2000,
			EndedAt:     int64Ptr(now - 1500),
			Status:      "completed",
		},
		{
			SessionID:   "abandoned-cache",
			Project:     project,
			FeatureName: stringPtr("caching"),
			StartedAt:   now - 1000,
			EndedAt:     int64Ptr(now - 500),
			Status:      "abandoned",
		},
	}

	for _, s := range sessions {
		err := CreateSession(db, s)
		require.NoError(t, err)
	}

	// Test: Get incomplete features
	incomplete, err := GetIncompleteFeatures(db, project)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(incomplete), 2) // Should include "active" and "abandoned"

	// Verify we have the expected features
	featureNames := make(map[string]bool)
	for _, session := range incomplete {
		if session.FeatureName != nil {
			featureNames[*session.FeatureName] = true
		}
	}
	assert.True(t, featureNames["authentication"])
	assert.True(t, featureNames["caching"])
}

// TestGetSessionContext tests getting full context for a session
func TestGetSessionContext(t *testing.T) {
	db := NewTestDBWithSchema(t)

	now := time.Now().UnixMilli()

	// Create a session
	session := &models.Session{
		SessionID:   "context-session",
		Project:     "/test/project",
		FeatureName: stringPtr("test-feature"),
		StartedAt:   now,
		Status:      "active",
	}
	err := CreateSession(db, session)
	require.NoError(t, err)

	// Test: Get session context
	context, err := GetSessionContext(db, session.SessionID)
	require.NoError(t, err)
	require.NotNil(t, context)

	// Verify session data
	assert.Equal(t, "context-session", context.Session.SessionID)
	assert.Equal(t, "/test/project", context.Session.Project)
	assert.NotNil(t, context.Session.FeatureName)
	assert.Equal(t, "test-feature", *context.Session.FeatureName)

	// Verify steps (should be empty for now)
	assert.NotNil(t, context.Steps)
	assert.Len(t, context.Steps, 0)

	// Verify metadata
	assert.Equal(t, int64(0), context.TotalSteps)
	assert.Equal(t, int64(0), context.TotalAgents)
}

// TestGetSessionContext_NotFound tests getting context for non-existent session
func TestGetSessionContext_NotFound(t *testing.T) {
	db := NewTestDBWithSchema(t)

	context, err := GetSessionContext(db, "nonexistent-session")
	assert.Error(t, err)
	assert.Nil(t, context)
}

// TestGetRecentSessionsGlobal tests getting recent sessions across all projects
func TestGetRecentSessionsGlobal(t *testing.T) {
	db := NewTestDBWithSchema(t)

	now := time.Now().UnixMilli()

	// Create sessions across multiple projects
	sessions := []*models.Session{
		{
			SessionID: "proj-a-1",
			Project:   "/project/a",
			StartedAt: now - 3000,
			EndedAt:   int64Ptr(now - 2500),
			Status:    "completed",
		},
		{
			SessionID: "proj-b-1",
			Project:   "/project/b",
			StartedAt: now - 2000,
			EndedAt:   int64Ptr(now - 1500),
			Status:    "completed",
		},
		{
			SessionID: "proj-a-2",
			Project:   "/project/a",
			StartedAt: now - 1000,
			Status:    "active",
		},
	}

	for _, s := range sessions {
		err := CreateSession(db, s)
		require.NoError(t, err)
	}

	// Test: Get 2 most recent globally
	recent, err := GetRecentSessionsGlobal(db, 2)
	require.NoError(t, err)
	assert.Len(t, recent, 2)
	assert.Equal(t, "proj-a-2", recent[0].SessionID) // Most recent
	assert.Equal(t, "proj-b-1", recent[1].SessionID)
}
