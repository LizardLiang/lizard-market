package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Routine represents a recurring personal ritual (global, not project-scoped)
type Routine struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	Cadence   string `json:"cadence"`   // daily | weekly:mon[,thu,...] | monthly:<1-28>
	LastDone  *int64 `json:"last_done"` // Unix epoch ms, null if never done
	CreatedAt int64  `json:"created_at"`
	DueToday  bool   `json:"due_today"` // computed at list time, not stored
}

var weekdayNames = map[string]time.Weekday{
	"mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday, "sun": time.Sunday,
}

// ValidateCadence checks a cadence spec: daily | weekly:<day[,day...]> | monthly:<1-28>
func ValidateCadence(cadence string) error {
	if cadence == "daily" {
		return nil
	}
	if days, ok := strings.CutPrefix(cadence, "weekly:"); ok {
		if days == "" {
			return fmt.Errorf("weekly cadence needs at least one day (e.g. weekly:mon,thu)")
		}
		for _, d := range strings.Split(days, ",") {
			if _, known := weekdayNames[d]; !known {
				return fmt.Errorf("invalid weekday %q in cadence (use mon..sun)", d)
			}
		}
		return nil
	}
	if day, ok := strings.CutPrefix(cadence, "monthly:"); ok {
		n, err := strconv.Atoi(day)
		if err != nil || n < 1 || n > 28 {
			return fmt.Errorf("monthly cadence needs a day of month 1-28 (got %q)", day)
		}
		return nil
	}
	return fmt.Errorf("invalid cadence %q (use daily, weekly:<days>, or monthly:<1-28>)", cadence)
}

// cadenceMatches reports whether the cadence falls on now's date
func cadenceMatches(cadence string, now time.Time) bool {
	if cadence == "daily" {
		return true
	}
	if days, ok := strings.CutPrefix(cadence, "weekly:"); ok {
		for _, d := range strings.Split(days, ",") {
			if wd, known := weekdayNames[d]; known && wd == now.Weekday() {
				return true
			}
		}
		return false
	}
	if day, ok := strings.CutPrefix(cadence, "monthly:"); ok {
		n, err := strconv.Atoi(day)
		return err == nil && n == now.Day()
	}
	return false
}

// routineDue reports whether a routine is due: its cadence falls on today
// AND it has not been done since the start of today (in now's location)
func routineDue(cadence string, lastDone *int64, now time.Time) bool {
	if !cadenceMatches(cadence, now) {
		return false
	}
	if lastDone == nil {
		return true
	}
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return *lastDone < startOfDay.UnixMilli()
}

// AddRoutine inserts a new routine after validating its cadence
func AddRoutine(db *sql.DB, text, cadence string) (*Routine, error) {
	if err := ValidateCadence(cadence); err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	result, err := db.Exec(
		`INSERT INTO routines (text, cadence, created_at) VALUES (?, ?, ?)`,
		text, cadence, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add routine: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert ID: %w", err)
	}

	return &Routine{
		ID:        id,
		Text:      text,
		Cadence:   cadence,
		CreatedAt: now,
	}, nil
}

// ListRoutines retrieves routines with DueToday computed against now,
// optionally filtered to only those due
func ListRoutines(db *sql.DB, now time.Time, dueOnly bool) ([]*Routine, error) {
	rows, err := db.Query(
		`SELECT id, text, cadence, last_done, created_at FROM routines ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list routines: %w", err)
	}
	defer rows.Close()

	var routines []*Routine
	for rows.Next() {
		r := &Routine{}
		if err := rows.Scan(&r.ID, &r.Text, &r.Cadence, &r.LastDone, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan routine: %w", err)
		}
		r.DueToday = routineDue(r.Cadence, r.LastDone, now)
		if dueOnly && !r.DueToday {
			continue
		}
		routines = append(routines, r)
	}
	return routines, rows.Err()
}

// DoneRoutine marks a routine as done now and returns the updated row
func DoneRoutine(db *sql.DB, id int64) (*Routine, error) {
	now := time.Now().UnixMilli()
	result, err := db.Exec("UPDATE routines SET last_done = ? WHERE id = ?", now, id)
	if err != nil {
		return nil, fmt.Errorf("failed to mark routine done: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("routine %d not found", id)
	}
	return GetRoutine(db, id)
}

// RemoveRoutine deletes a routine by ID
func RemoveRoutine(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM routines WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to remove routine: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("routine %d not found", id)
	}
	return nil
}

// GetRoutine retrieves a single routine by ID (DueToday computed against local now)
func GetRoutine(db *sql.DB, id int64) (*Routine, error) {
	r := &Routine{}
	err := db.QueryRow(
		`SELECT id, text, cadence, last_done, created_at FROM routines WHERE id = ?`, id,
	).Scan(&r.ID, &r.Text, &r.Cadence, &r.LastDone, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("routine %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get routine: %w", err)
	}
	r.DueToday = routineDue(r.Cadence, r.LastDone, time.Now())
	return r, nil
}
