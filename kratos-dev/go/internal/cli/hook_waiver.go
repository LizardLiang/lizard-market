package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// The user-speed waiver. When the user asks for speed, no tests or no commit,
// the orchestrator (Iris) puts one line in the Ares mission, or in a
// SendMessage to a running Ares:
//
//	USER-WAIVER: speed — "<the user's verbatim words>"
//
// aresReportFailures then skips the Final Block checks that the user's words
// override (test run, RED/GREEN, Reach, reach child, task list, completion,
// Landed, the comment gate) and keeps the files check, which a no-code report
// (LANDED-NOT-APPLICABLE plus only `Reach: not applicable`) also passes. 2026-10-28 and 10-29, three runs: the gates
// made Ares spawn a reach child and run tests against the user's instruction.
//
// Only an inbound entry of the Ares transcript can grant the waiver: the first
// user entry (the mission) or a queued_command attachment from the coordinator
// (a SendMessage). The text Ares writes never counts. The quoted words must also
// appear in a human prompt of the parent transcript, so a waiver the
// orchestrator invented is refused.

var userWaiverLineRE = regexp.MustCompile(`(?im)^[ \t]*USER-WAIVER:[ \t]*speed[ \t]*[—–-][ \t]*["“'](.+?)["”']?[ \t\r]*$`)

// waiverEntry is the subset of a transcript entry the waiver scan reads.
type waiverEntry struct {
	Type    string `json:"type"`
	Message *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Origin *struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	Attachment *struct {
		Type   string          `json:"type"`
		Prompt json.RawMessage `json:"prompt"`
		Origin *struct {
			Kind string `json:"kind"`
		} `json:"origin"`
	} `json:"attachment"`
}

// waiverText joins the text of a message.content or attachment.prompt value,
// whether it is a plain string or an array of text blocks.
func waiverText(raw json.RawMessage) string {
	if s, ok := handbackRawString(raw); ok {
		return s
	}
	var parts []string
	for _, b := range handbackParseBlocks(raw) {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// forEachWaiverEntry calls fn for every parseable JSONL entry of path until fn
// returns true.
func forEachWaiverEntry(path string, fn func(waiverEntry) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var e waiverEntry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if fn(e) {
			return nil
		}
	}
	return sc.Err()
}

// inboundWaiverQuotes returns the quoted words of every USER-WAIVER line in an
// inbound entry of the Ares transcript at path.
func inboundWaiverQuotes(path string) ([]string, error) {
	var quotes []string
	missionSeen := false
	err := forEachWaiverEntry(path, func(e waiverEntry) bool {
		var text string
		switch {
		case e.Type == "user" && e.Message != nil && !missionSeen:
			missionSeen = true
			text = waiverText(e.Message.Content)
		case e.Type == "attachment" && e.Attachment != nil && e.Attachment.Type == "queued_command" &&
			e.Attachment.Origin != nil && e.Attachment.Origin.Kind == "coordinator":
			text = waiverText(e.Attachment.Prompt)
		}
		for _, m := range userWaiverLineRE.FindAllStringSubmatch(text, -1) {
			quotes = append(quotes, m[1])
		}
		return false
	})
	return quotes, err
}

// waiverMinQuoteRunes is the shortest quote the gate accepts: a one-letter
// quote would match any human prompt.
const waiverMinQuoteRunes = 8

func normalizeWaiverQuote(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.Trim(s, "\"“”' ")), " "))
}

// humanPromptsHave reports whether any human prompt of the transcript at path
// contains quote. known is false when the transcript is unreadable or holds no
// human prompt, so the caller fails open.
func humanPromptsHave(path, quote string) (found, known bool) {
	want := normalizeWaiverQuote(quote)
	if len([]rune(want)) < waiverMinQuoteRunes {
		return false, true
	}
	err := forEachWaiverEntry(path, func(e waiverEntry) bool {
		if e.Type != "user" || e.Origin == nil || e.Origin.Kind != "human" || e.Message == nil {
			return false
		}
		known = true
		if strings.Contains(normalizeWaiverQuote(waiverText(e.Message.Content)), want) {
			found = true
			return true
		}
		return false
	})
	if err != nil {
		return false, false
	}
	return found, known
}

// aresUserWaiverGranted reports whether the Ares transcript carries a valid
// USER-WAIVER line. An unreadable Ares transcript grants nothing. An unreadable
// parent transcript, or one with no human prompt, cannot refute the quote, so
// the gate fails open on that check like every other check.
func aresUserWaiverGranted(input subagentStopInput) bool {
	agentPath := subagentHandbackTranscriptPath(input)
	if agentPath == "" {
		return false
	}
	quotes, err := inboundWaiverQuotes(agentPath)
	if err != nil {
		debugLog("user waiver: transcript scan failed: %v", err)
		return false
	}
	if len(quotes) == 0 {
		return false
	}
	parent := input.TranscriptPath
	if parent == "" || parent == agentPath {
		return true
	}
	for _, q := range quotes {
		found, known := humanPromptsHave(parent, q)
		if !known || found {
			return true
		}
	}
	return false
}
