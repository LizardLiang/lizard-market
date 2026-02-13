package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// TestGetRecentSessions tests retrieving recent sessions across all projects
func TestGetRecentSessions(t *testing.T) {
	db := NewTestDBWithSchema(t)

	// Create sessions across multiple projects
	now := time.Now().UnixMilli()
	sessions := []*models.Session{
		{
			SessionID: "sess-1",
			Project:   "/project/a",
			StartedAt: now - 3000,
			Status:    "completed",
		},
		{
			SessionID: "sess-2",
			Project:   "/project/b",
			StartedAt: now - 2000,
			Status:    "active",
		},
		{
			SessionID: "sess-3",
			Project:   "/project/a",
			StartedAt: now - 1000,
			Status:    "active",
		},
	}

	for _, s := range sessions {
		err := CreateSession(db, s)
		require.NoError(t, err)
	}

	// Test: Get 2 most recent sessions (across all projects)
	recent, err := GetRecentSessions(db, 2)
	require.NoError(t, err)
	assert.Len(t, recent, 2)
	assert.Equal(t, "sess-3", recent[0].SessionID) // Most recent
	assert.Equal(t, "sess-2", recent[1].SessionID)
}

// TestGetSessionsByStatus tests filtering sessions by status
func TestGetSessionsByStatus(t *testing.T) {
	db := NewTestDBWithSchema(t)

	now := time.Now().UnixMilli()

	// Create sessions with different statuses
	sessions := []*models.Session{
		{
			SessionID: "active-1",
			Project:   "/project/a",
			StartedAt: now - 3000,
			Status:    "active",
		},
		{
			SessionID: "completed-1",
			Project:   "/project/a",
			StartedAt: now - 2000,
			EndedAt:   int64Ptr(now - 1000),
			Status:    "completed",
		},
		{
			SessionID: "active-2",
			Project:   "/project/b",
			StartedAt: now - 1000,
			Status:    "active",
		},
		{
			SessionID: "abandoned-1",
			Project:   "/project/c",
			StartedAt: now - 500,
			Status:    "abandoned",
		},
	}

	for _, s := range sessions {
		err := CreateSession(db, s)
		require.NoError(t, err)
	}

	// Test: Get active sessions
	activeSessions, err := GetSessionsByStatus(db, "active")
	require.NoError(t, err)
	assert.Len(t, activeSessions, 2)

	// Test: Get completed sessions
	completedSessions, err := GetSessionsByStatus(db, "completed")
	require.NoError(t, err)
	assert.Len(t, completedSessions, 1)
	assert.Equal(t, "completed-1", completedSessions[0].SessionID)

	// Test: Get abandoned sessions
	abandonedSessions, err := GetSessionsByStatus(db, "abandoned")
	require.NoError(t, err)
	assert.Len(t, abandonedSessions, 1)
}

// TestSearchSessions tests FTS5 full-text search across sessions
func TestSearchSessions(t *testing.T) {
	db := NewTestDBWithSchema(t)

	now := time.Now().UnixMilli()

	// Create sessions with searchable content
	sessions := []*models.Session{
		{
			SessionID:   "auth-sess",
			Project:     "/project/api",
			FeatureName: stringPtr("authentication"),
			StartedAt:   now - 3000,
			Status:      "completed",
			Summary:     stringPtr("Implemented OAuth2 authentication with JWT tokens"),
		},
		{
			SessionID:   "payment-sess",
			Project:     "/project/api",
			FeatureName: stringPtr("payment-gateway"),
			StartedAt:   now - 2000,
			Status:      "active",
			Summary:     stringPtr("Integrated Stripe payment processing"),
		},
		{
			SessionID:   "cache-sess",
			Project:     "/project/backend",
			FeatureName: stringPtr("caching"),
			StartedAt:   now - 1000,
			Status:      "completed",
			Summary:     stringPtr("Added Redis caching layer for API responses"),
		},
	}

	for _, s := range sessions {
		err := CreateSession(db, s)
		require.NoError(t, err)
	}

	// Test: Search for "authentication"
	authResults, err := SearchSessions(db, "authentication")
	require.NoError(t, err)
	assert.Len(t, authResults, 1)
	assert.Equal(t, "auth-sess", authResults[0].SessionID)

	// Test: Search for "API"
	apiResults, err := SearchSessions(db, "api")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(apiResults), 2) // Should match project paths

	// Test: Search for "payment"
	paymentResults, err := SearchSessions(db, "payment")
	require.NoError(t, err)
	assert.Len(t, paymentResults, 1)
	assert.Equal(t, "payment-sess", paymentResults[0].SessionID)

	// Test: Search for non-existent term
	noResults, err := SearchSessions(db, "nonexistent-term-12345")
	require.NoError(t, err)
	assert.Len(t, noResults, 0)
}

// TestGetSessionTimeline tests retrieving all steps for a session
func TestGetSessionTimeline(t *testing.T) {
	db := NewTestDBWithSchema(t)

	// Create a session
	now := time.Now().UnixMilli()
	session := &models.Session{
		SessionID: "timeline-sess",
		Project:   "/project/test",
		StartedAt: now,
		Status:    "active",
	}
	err := CreateSession(db, session)
	require.NoError(t, err)

	// Insert steps (need to add CreateStep function eventually)
	// For now, test that the query structure works
	steps, err := GetSessionTimeline(db, session.SessionID)
	require.NoError(t, err)
	assert.NotNil(t, steps)
	assert.Len(t, steps, 0) // No steps yet
}

// TestGetSessionsByProject tests getting all sessions for a specific project
func TestGetSessionsByProject(t *testing.T) {
	db := NewTestDBWithSchema(t)

	now := time.Now().UnixMilli()

	// Create sessions for multiple projects
	sessions := []*models.Session{
		{
			SessionID: "proj-a-1",
			Project:   "/project/a",
			StartedAt: now - 3000,
			Status:    "completed",
		},
		{
			SessionID: "proj-a-2",
			Project:   "/project/a",
			StartedAt: now - 2000,
			Status:    "active",
		},
		{
			SessionID: "proj-b-1",
			Project:   "/project/b",
			StartedAt: now - 1000,
			Status:    "active",
		},
	}

	for _, s := range sessions {
		err := CreateSession(db, s)
		require.NoError(t, err)
	}

	// Test: Get all sessions for project A
	projectASessions, err := GetSessionsByProject(db, "/project/a")
	require.NoError(t, err)
	assert.Len(t, projectASessions, 2)

	// Test: Get all sessions for project B
	projectBSessions, err := GetSessionsByProject(db, "/project/b")
	require.NoError(t, err)
	assert.Len(t, projectBSessions, 1)

	// Test: Get sessions for non-existent project
	noSessions, err := GetSessionsByProject(db, "/project/nonexistent")
	require.NoError(t, err)
	assert.Len(t, noSessions, 0)
}

// TestGetSessionCount tests counting total sessions
func TestGetSessionCount(t *testing.T) {
	db := NewTestDBWithSchema(t)

	// Initially should be 0
	count, err := GetSessionCount(db)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Create 3 sessions
	now := time.Now().UnixMilli()
	for i := 1; i <= 3; i++ {
		session := &models.Session{
			SessionID: fmt.Sprintf("sess-%d", i),
			Project:   "/project/test",
			StartedAt: now + int64(i)*1000,
			Status:    "active",
		}
		err := CreateSession(db, session)
		require.NoError(t, err)
	}

	// Should now be 3
	count, err = GetSessionCount(db)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}
