package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSession_JSONMarshaling tests marshaling Session to JSON
func TestSession_JSONMarshaling(t *testing.T) {
	featureName := "auth-system"
	summary := "Implemented OAuth2"

	session := Session{
		ID:                 1,
		SessionID:          "sess-123",
		Project:            "/path/to/project",
		FeatureName:        &featureName,
		StartedAt:          1707738000,
		EndedAt:            nil,
		Status:             "active",
		Summary:            &summary,
		TotalSteps:         5,
		TotalAgentsSpawned: 2,
	}

	// Marshal
	jsonBytes, err := json.Marshal(session)
	require.NoError(t, err)

	// Verify JSON structure
	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(1), result["id"])
	assert.Equal(t, "sess-123", result["session_id"])
	assert.Equal(t, "auth-system", result["feature_name"])
	assert.Equal(t, "Implemented OAuth2", result["summary"])
	assert.Nil(t, result["ended_at"])
}

// TestSession_JSONUnmarshaling tests unmarshaling JSON to Session
func TestSession_JSONUnmarshaling(t *testing.T) {
	jsonStr := `{
		"id": 1,
		"session_id": "sess-123",
		"project": "/path/to/project",
		"feature_name": "auth-system",
		"started_at": 1707738000,
		"ended_at": null,
		"status": "active",
		"summary": null,
		"total_steps": 5,
		"total_agents_spawned": 2
	}`

	var session Session
	err := json.Unmarshal([]byte(jsonStr), &session)
	require.NoError(t, err)

	assert.Equal(t, int64(1), session.ID)
	assert.Equal(t, "sess-123", session.SessionID)
	assert.NotNil(t, session.FeatureName)
	assert.Equal(t, "auth-system", *session.FeatureName)
	assert.Nil(t, session.EndedAt)
	assert.Nil(t, session.Summary)
}

// TestSession_NullOptionalFields tests Session with null optional fields
func TestSession_NullOptionalFields(t *testing.T) {
	session := Session{
		ID:                 1,
		SessionID:          "sess-123",
		Project:            "/path/to/project",
		FeatureName:        nil, // NULL
		StartedAt:          1707738000,
		EndedAt:            nil, // NULL
		Status:             "active",
		Summary:            nil, // NULL
		TotalSteps:         0,
		TotalAgentsSpawned: 0,
	}

	jsonBytes, err := json.Marshal(session)
	require.NoError(t, err)

	// Verify omitempty works for nil pointers
	jsonStr := string(jsonBytes)
	assert.NotContains(t, jsonStr, "feature_name")
	assert.NotContains(t, jsonStr, "ended_at")
	assert.NotContains(t, jsonStr, "summary")
}
