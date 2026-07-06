package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitDB_CreatesRoutinesTable verifies the schema upgrade: InitDB run
// against a database created before this change gains the routines table.
func TestInitDB_CreatesRoutinesTable(t *testing.T) {
	db := NewTestDB(t)

	err := InitDB(db)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='routines'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "routines table should exist")
}

func TestValidateCadence(t *testing.T) {
	valid := []string{"daily", "weekly:mon", "weekly:mon,thu", "weekly:sat,sun", "monthly:1", "monthly:15", "monthly:28"}
	for _, c := range valid {
		assert.NoError(t, ValidateCadence(c), "cadence %q should be valid", c)
	}

	invalid := []string{"", "hourly", "weekly:", "weekly:funday", "weekly:mon,funday", "monthly:", "monthly:0", "monthly:29", "monthly:31", "monthly:abc"}
	for _, c := range invalid {
		assert.Error(t, ValidateCadence(c), "cadence %q should be invalid", c)
	}
}

func TestRoutineDue(t *testing.T) {
	// 2024-01-15 was a Monday; 2024-01-18 a Thursday
	monday := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	tuesday := time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC)
	thursday := time.Date(2024, 1, 18, 10, 0, 0, 0, time.UTC)
	the15th := monday
	the16th := tuesday

	doneEarlierToday := monday.Add(-2 * time.Hour).UnixMilli()
	doneYesterday := monday.Add(-24 * time.Hour).UnixMilli()

	cases := []struct {
		name     string
		cadence  string
		lastDone *int64
		now      time.Time
		want     bool
	}{
		{"daily never done", "daily", nil, monday, true},
		{"daily done today", "daily", &doneEarlierToday, monday, false},
		{"daily done yesterday", "daily", &doneYesterday, monday, true},
		{"weekly:mon on Tuesday", "weekly:mon", nil, tuesday, false},
		{"weekly:mon,thu on Thursday", "weekly:mon,thu", nil, thursday, true},
		{"weekly:mon done earlier same Monday", "weekly:mon", &doneEarlierToday, monday, false},
		{"monthly:15 on the 15th", "monthly:15", nil, the15th, true},
		{"monthly:15 on the 16th", "monthly:15", nil, the16th, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, routineDue(tc.cadence, tc.lastDone, tc.now))
		})
	}
}

func TestAddRoutine(t *testing.T) {
	db := NewTestDBWithSchema(t)

	r, err := AddRoutine(db, "review inbox", "weekly:mon")
	require.NoError(t, err)
	assert.NotZero(t, r.ID)
	assert.Equal(t, "review inbox", r.Text)
	assert.Equal(t, "weekly:mon", r.Cadence)
	assert.Nil(t, r.LastDone)
	assert.NotZero(t, r.CreatedAt)
}

func TestAddRoutine_RejectsInvalidCadence(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := AddRoutine(db, "bad", "weekly:funday")
	assert.Error(t, err)
}

func TestListRoutines_DueOnly(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := AddRoutine(db, "daily standup notes", "daily")
	require.NoError(t, err)
	_, err = AddRoutine(db, "monday review", "weekly:mon")
	require.NoError(t, err)

	// 2024-01-16 was a Tuesday: only the daily routine is due
	tuesday := time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC)
	due, err := ListRoutines(db, tuesday, true)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, "daily standup notes", due[0].Text)

	all, err := ListRoutines(db, tuesday, false)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestDoneRoutine_ClearsDueToday(t *testing.T) {
	db := NewTestDBWithSchema(t)

	r, err := AddRoutine(db, "daily standup notes", "daily")
	require.NoError(t, err)

	done, err := DoneRoutine(db, r.ID)
	require.NoError(t, err)
	require.NotNil(t, done.LastDone)

	due, err := ListRoutines(db, time.Now(), true)
	require.NoError(t, err)
	assert.Len(t, due, 0, "routine done now should not be due today")
}

func TestDoneRoutine_NotFound(t *testing.T) {
	db := NewTestDBWithSchema(t)

	_, err := DoneRoutine(db, 999)
	assert.Error(t, err)
}

func TestRemoveRoutine(t *testing.T) {
	db := NewTestDBWithSchema(t)

	r, err := AddRoutine(db, "review inbox", "daily")
	require.NoError(t, err)

	err = RemoveRoutine(db, r.ID)
	require.NoError(t, err)

	_, err = GetRoutine(db, r.ID)
	assert.Error(t, err, "routine should no longer exist after removal")
}

func TestRemoveRoutine_NotFound(t *testing.T) {
	db := NewTestDBWithSchema(t)

	err := RemoveRoutine(db, 999)
	assert.Error(t, err)
}
