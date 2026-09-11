package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
)

// lessonsInjectMax caps how many stored lessons an inline god sees at load.
const lessonsInjectMax = 5

// retroNudgeAt is the pending-lesson count from which `agent load` reminds
// the user to fold them with /kratos:retro.
const retroNudgeAt = 5

// lessonsBlockFor returns the "lessons from past user corrections" block for
// an inline (command-mode) god, or "" when there are none or the store is
// unavailable. Spawned subagents already receive this block from
// hooks/path-inject.cjs at SubagentStart; inline gods (Odysseus, Iris, Themis,
// Hephaestus ANALYZE) never had a SubagentStart, so 13 Odysseus lessons sat
// unused for two months (2026-09 review). Current-project lessons sort first.
func lessonsBlockFor(god string) string {
	god = strings.ToLower(strings.TrimSpace(god))
	if god == "" {
		return ""
	}
	conn, err := db.GetConnection()
	if err != nil {
		return ""
	}
	defer conn.Close()
	if err := db.InitDB(conn); err != nil {
		return ""
	}

	project := ""
	if cwd, err := os.Getwd(); err == nil {
		project = filepath.Base(cwd)
	}
	all, err := db.ListFeedback(conn, god, project, 0)
	if err != nil || len(all) == 0 {
		return ""
	}
	lessons := make([]string, 0, len(all))
	for _, f := range all {
		lessons = append(lessons, f.Lesson)
	}
	return renderLessonsBlock(god, lessons)
}

// renderLessonsBlock formats the first lessonsInjectMax of lessons for god and
// appends the retro nudge once the pending total reaches retroNudgeAt. Pure —
// no store — so the inline-budget lint can render a worst-case block.
func renderLessonsBlock(god string, lessons []string) string {
	if len(lessons) == 0 {
		return ""
	}
	shown := lessons
	if len(shown) > lessonsInjectMax {
		shown = shown[:lessonsInjectMax]
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Lessons from past user corrections of %s** — apply them to this task:\n", god)
	for _, l := range shown {
		fmt.Fprintf(&sb, "- %s\n", l)
	}
	if len(lessons) >= retroNudgeAt {
		fmt.Fprintf(&sb, "\n(%d lessons pending — `/kratos:retro %s` folds the stable ones into this definition.)\n", len(lessons), god)
	}
	return strings.TrimRight(sb.String(), "\n")
}
