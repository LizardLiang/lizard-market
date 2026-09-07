package cli

import (
	"time"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
)

// profileStaleAfter is the age from which a profile slot is reported as stale.
// Iris treats a stale slot as unknown instead of advising from it: every slot
// in the store was two months old and still fed the daily briefing (2026-09).
const profileStaleAfter = 30 * 24 * time.Hour

// profileEntryOut is a profile entry with its age and staleness for callers.
type profileEntryOut struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt int64  `json:"updated_at"`
	AgeDays   int    `json:"age_days"`
	Stale     bool   `json:"stale,omitempty"`
}

// withStaleFlags annotates entries with age_days and stale (updated more than
// profileStaleAfter ago) relative to now.
func withStaleFlags(entries []*db.ProfileEntry, now time.Time) []profileEntryOut {
	out := make([]profileEntryOut, 0, len(entries))
	for _, e := range entries {
		age := now.Sub(time.UnixMilli(e.UpdatedAt))
		out = append(out, profileEntryOut{
			Key:       e.Key,
			Value:     e.Value,
			UpdatedAt: e.UpdatedAt,
			AgeDays:   int(age.Hours() / 24),
			Stale:     age > profileStaleAfter,
		})
	}
	return out
}
