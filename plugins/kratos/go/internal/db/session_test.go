package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/lizard-market/plugins/kratos/internal/models"
)

// Helper functions for pointer types
func stringPtr(s string) *string { return &s }
func int64Ptr(i int64) *int64    { return &i }

// Test 1: Create session
func TestCreateSession(t *testing.T) {
	db := NewTestDBWithSchema(t)

	session := &models.Session{
		SessionID:          "test-session-123",
		Project:            "/path/to/project",
		FeatureName:        stringPtr("auth-feature"),
		StartedAt:          time.Now().UnixMilli(),
		Status:             "active",
		TotalSteps:         0,
		TotalAgentsSpawned: 0,
	}

	err := CreateSession(db, session)
	require.NoError(t, err)
	assert.Greater(t, session.ID, int64(0), "ID should be set")
}

// Test 2: Get session by ID
func TestGetSession(t *testing.T) {
	db := NewTestDBWithSchema(t)

	// Setup: Create a session
	original := &models.Session{
		SessionID: "get-test-123",
		Project:   "/test/project",
		StartedAt: time.Now().UnixMilli(),
		Status:    "active",
	}
	err := CreateSession(db, original)
	require.NoError(t, err)

	// Test: Retrieve it
	retrieved, err := GetSession(db, original.SessionID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, original.SessionID, retrieved.SessionID)
	assert.Equal(t, original.Project, retrieved.Project)
}

// Test 3: Get session not found
func TestGetSession_NotFound(t *testing.T) {
	db := NewTestDBWithSchema(t)

	session, err := GetSession(db, "nonexistent-id")
	assert.Error(t, err)
	assert.Nil(t, session)
}

// Test 4: Get active session for project
func TestGetActiveSession(t *testing.T) {
	db := NewTestDBWithSchema(t)
	project := "/test/project"

	// Create active session
	active := &models.Session{
		SessionID: "active-123",
		Project:   project,
		StartedAt: time.Now().UnixMilli(),
		Status:    "active",
	}
	err := CreateSession(db, active)
	require.NoError(t, err)

	// Create completed session (should be ignored)
	completed := &models.Session{
		SessionID: "completed-123",
		Project:   project,
		StartedAt: time.Now().UnixMilli(),
		EndedAt:   int64Ptr(time.Now().UnixMilli()),
		Status:    "completed",
	}
	err = CreateSession(db, completed)
	require.NoError(t, err)

	// Test: Should get only active
	retrieved, err := GetActiveSession(db, project)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "active-123", retrieved.SessionID)
}

// Test 5: End session
func TestEndSession(t *testing.T) {
	db := NewTestDBWithSchema(t)

	// Create active session
	session := &models.Session{
		SessionID: "end-test-123",
		Project:   "/test/project",
		StartedAt: time.Now().UnixMilli(),
		Status:    "active",
	}
	err := CreateSession(db, session)
	require.NoError(t, err)

	// Test: End session
	summary := "Test complete"
	err = EndSession(db, session.SessionID, summary)
	require.NoError(t, err)

	// Verify: Check updated
	retrieved, err := GetSession(db, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, "completed", retrieved.Status)
	assert.NotNil(t, retrieved.EndedAt)
	assert.NotNil(t, retrieved.Summary)
	assert.Equal(t, summary, *retrieved.Summary)
}

// Test 6: List recent sessions
func TestListRecentSessions(t *testing.T) {
	db := NewTestDBWithSchema(t)
	project := "/test/project"

	// Create 3 sessions with distinct timestamps
	baseTime := time.Now().UnixMilli()
	for i := 1; i <= 3; i++ {
		session := &models.Session{
			SessionID: fmt.Sprintf("session-%d", i),
			Project:   project,
			StartedAt: baseTime + int64(i)*1000,
			Status:    "active",
		}
		err := CreateSession(db, session)
		require.NoError(t, err)
	}

	// Test: Get 2 most recent
	sessions, err := ListRecentSessions(db, project, 2)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
	assert.Equal(t, "session-3", sessions[0].SessionID) // Most recent first
	assert.Equal(t, "session-2", sessions[1].SessionID)
}
