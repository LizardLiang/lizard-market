package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// The Hermes hand-back gate. One PreToolUse hook, matched on the
// SubagentHandback tool, stops Hermes from delivering its verdict before its
// own review children have reported.
//
// Hermes spawns three review children (and, after that, validation agents)
// through the Task tool. Every one of those spawns is async: the tool result
// is "Async agent launched successfully", not the child's findings. A child's
// real report arrives later, as an <agent-message from="…"> entry in
// Hermes's own transcript, and that arrival resumes Hermes. Nothing stopped
// Hermes from calling SubagentHandback the moment its own first pass ended,
// before any child had reported — the NETZERO 09-17 incident, where two
// confirmed BLOCKERs never reached the caller.
//
// This gate reads Hermes's own subagent transcript, matches each launch to
// its own report or terminal notification, and denies the hand-back while
// any launched child is still outstanding — UNLESS the transcript shows no
// progress for handbackStallTimeout, in which case it releases the hand-back
// (no decision) with a visible additionalContext naming who never reported.
// It never allows outright, and it holds no state of its own: every call
// recomputes its verdict from the transcript and the clock. The stateless
// redesign replaced a denial counter that counted consecutive DENIALS, not
// elapsed time — three retries of one denied tool call (routine for a model)
// tripped it and reopened the gate with all three children still outstanding,
// reproducing the 09-17 incident through the gate meant to stop it.
//
// Counting is scoped to JSON structure, not raw substring search. Hermes
// itself, or one of its children, can Read or Grep this very file, this
// plan, or a fixture that quotes "Async agent launched successfully" or
// <agent-message from="…"> in plain prose — a raw text scan over the
// transcript would count that quoted text as a real launch or report and
// deny every hand-back forever. Scoping a launch to a tool_result whose
// tool_use_id answers a real Agent/Task tool_use, and scoping a report to a
// non-tool-result entry, keeps an unrelated Read/Grep result inert.

// handbackStallTimeout bounds how long this gate keeps denying with no
// forward motion. Real data for calibration (09-17 NETZERO transcript):
// children finished 11 to 22 minutes after launch, and the longest gap
// between two sibling finishes was 8 minutes — 30 minutes clears both with
// room to spare before the gate lets a hand-back through with children still
// missing.
const handbackStallTimeout = 30 * time.Minute

// handbackNow is the clock this gate reads, a package variable so a test can
// inject a fixed time instead of racing the real one.
var handbackNow = time.Now

// handbackAsyncLaunchMarker is the literal text the harness returns as a tool
// result when a subagent spawns a child asynchronously.
const handbackAsyncLaunchMarker = "Async agent launched successfully"

// handbackLaunchIDRE pairs a launch marker with the child's agent id, printed
// on the same transcript line as "agentId: <id>". Used only as a fallback —
// scanHandbackTranscript prefers the structured toolUseResult.agentId field
// on the same JSONL entry when it is present.
var handbackLaunchIDRE = regexp.MustCompile(`agentId:\s*([A-Za-z0-9_-]+)`)

// handbackReportRE matches a child's hand-back arriving in the parent's own
// transcript: <agent-message from="<agentId>">. Operates on already
// JSON-decoded text, where a `"` is a real quote character, not `\"`.
var handbackReportRE = regexp.MustCompile(`<agent-message from="([^"]+)">`)

// handbackTaskNotificationRE pulls the task id out of a <task-notification>
// block, regardless of its <status>. A child that crashed, was stopped, or
// otherwise never sends an <agent-message> still ends with one of these —
// without it, a dead child would leave the gate denying forever.
var handbackTaskNotificationRE = regexp.MustCompile(`(?s)<task-notification>.*?<task-id>([^<]+)</task-id>`)

func handbackGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "handback-gate",
		Short: "Handle PreToolUse for SubagentHandback — deny Hermes's hand-back until every launched child has finished",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				debugLog("handback-gate: stdin read error: %v", err)
				return nil
			}
			handleHandbackGate(raw)
			return nil
		},
	}
}

// handleHandbackGate runs the gate for one raw PreToolUse payload and prints
// the decision, if any. Every error path is silent and non-blocking, the
// same contract handleEditGate keeps.
func handleHandbackGate(raw []byte) {
	var input preToolUseInput
	if err := json.Unmarshal(raw, &input); err != nil {
		debugLog("handback-gate: json parse error: %v", err)
		return
	}

	res := handbackGateDecision(input)
	emitPreToolUseDecision(res.Decision, res.Reason, res.AdditionalContext)
}

// handbackGateResult is one gate verdict. Decision "" means the tool call is
// not blocked — either nothing to say at all, or the stall-timeout release
// below, which still has something to say without blocking it.
type handbackGateResult struct {
	Decision string // "" (no decision) or "deny"
	Reason   string
	// AdditionalContext is set only on the stall-timeout release: Decision
	// stays "" (the hand-back proceeds), but the missing children are named
	// so Hermes's report can mark their tiers parent-only, not child-verified.
	AdditionalContext string
}

// handbackGateDecision is the whole policy, as a pure function of the
// payload and the clock (handbackNow), so the table test can exercise every
// branch without touching a real transcript for the "no decision" cases, and
// a time-based test can inject a fixed clock instead of racing the real one.
func handbackGateDecision(input preToolUseInput) handbackGateResult {
	if input.ToolName != "SubagentHandback" {
		return handbackGateResult{}
	}
	if !strings.Contains(strings.ToLower(input.AgentType), "hermes") {
		return handbackGateResult{}
	}

	path := handbackTranscriptPath(input)
	if path == "" {
		return handbackGateResult{}
	}

	launchedIDs, finishedIDs, lastProgress, hasProgress, err := scanHandbackTranscript(path)
	if err != nil {
		debugLog("handback-gate: transcript read error for %s: %v", path, err)
		return handbackGateResult{}
	}

	var missing []string
	for id := range launchedIDs {
		if !finishedIDs[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return handbackGateResult{}
	}
	sort.Strings(missing)

	if !hasProgress {
		// No launch or finish event anywhere in the transcript carries a
		// parseable timestamp, so the stall clock has nothing to measure
		// against — this gate cannot tell a fresh launch from a genuine
		// stall. Fail open, the same discipline an unreadable transcript
		// gets above, rather than deny with no way to ever release.
		debugLog("handback-gate: no usable timestamp in %s, failing open with %d missing", path, len(missing))
		return handbackGateResult{}
	}
	if handbackNow().Sub(lastProgress) >= handbackStallTimeout {
		return handbackGateResult{AdditionalContext: handbackStallContext(missing)}
	}
	return handbackGateResult{Decision: "deny", Reason: handbackDenyReason(missing)}
}

// handbackTranscriptPath resolves Hermes's own subagent transcript. The
// payload's transcript_path names the calling session's transcript file;
// Claude Code writes every spawned subagent's own transcript as a sibling
// directory next to it: <dir>/<session_id>/subagents/agent-<agent_id>.jsonl.
// Any missing field fails open — this gate must never guess a path.
func handbackTranscriptPath(input preToolUseInput) string {
	if input.TranscriptPath == "" || input.SessionID == "" || input.AgentID == "" {
		return ""
	}
	dir := filepath.Dir(input.TranscriptPath)
	return filepath.Join(dir, input.SessionID, "subagents", "agent-"+input.AgentID+".jsonl")
}

// handbackBlock is one content block inside a transcript entry's
// message.content array — an assistant's tool_use, a user's tool_result, or
// a plain text block. All three shapes share this one loose struct; a field
// a given block type does not carry is simply left at its zero value.
type handbackBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`          // tool_use
	Name      string          `json:"name"`        // tool_use
	ToolUseID string          `json:"tool_use_id"` // tool_result
	Text      string          `json:"text"`        // text
	Content   json.RawMessage `json:"content"`     // tool_result's own nested content
}

// handbackLine is the subset of one transcript JSONL entry this gate reads.
//
// Origin is Claude Code's own structured peer hand-back marker. Ground truth
// from a real Hermes transcript (09-17 NETZERO, agent-ae8cddbbbfd689d03.jsonl
// — shape only, never its text): it sits at the TOP LEVEL of a "user" entry
// whose message content is a plain string in 2 of 3 real reports, and nested
// under attachment.origin on a "queued_command" attachment in the third.
// Reading only the attachment shape missed the first two entirely.
type handbackLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Origin    *struct {
		Kind     string `json:"kind"`
		From     string `json:"from"`
		Handback bool   `json:"handback"`
	} `json:"origin"`
	Message *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Attachment *struct {
		// Prompt is json.RawMessage, not string: a real queued_command
		// attachment can carry it as a content-block array rather than a
		// plain string. A typed string field made json.Unmarshal fail on
		// that shape, and this gate skips a line it cannot parse — silently
		// dropping this attachment's own origin.handback along with it.
		Prompt json.RawMessage `json:"prompt"`
		Origin *struct {
			From     string `json:"from"`
			Handback bool   `json:"handback"`
		} `json:"origin"`
	} `json:"attachment"`
	// ToolUseResult carries the harness's own structured record of an async
	// spawn on the same line as its tool_result — a more reliable source for
	// the child's id than parsing prose, when present.
	ToolUseResult *struct {
		AgentID string `json:"agentId"`
	} `json:"toolUseResult"`
}

// parseHandbackTimestamp parses a transcript entry's own timestamp field
// (RFC 3339, e.g. "2026-09-17T02:51:32.741Z" — the shape every real Claude
// Code transcript line carries). ok is false for an empty or unparseable
// value, which scanHandbackTranscript treats as "no progress signal here",
// not as a parse failure worth logging.
func parseHandbackTimestamp(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// scanHandbackTranscript walks Hermes's own subagent transcript once and
// returns which children it launched, which of those have finished — either
// by reporting (<agent-message from="…"> or the structured Origin marker) or
// by a terminal <task-notification> naming their id, whatever that
// notification's status — and the newest timestamp among the launch and
// finish events themselves (hasProgress is false when none carried a
// parseable one). A finish is only counted for an id this same transcript
// actually launched; an unrelated id appearing in some other message never
// counts.
func scanHandbackTranscript(path string) (launchedIDs, finishedIDs map[string]bool, lastProgress time.Time, hasProgress bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, time.Time{}, false, err
	}
	defer f.Close()

	launchedIDs = map[string]bool{}
	finishedIDs = map[string]bool{}
	// pendingLaunchToolUseIDs holds the tool_use id of every Agent/Task call
	// Hermes has made so far, so a later tool_result can be recognized as
	// answering one of THOSE calls specifically — never a Read, Grep, or Bash
	// result that merely happens to quote the marker text.
	pendingLaunchToolUseIDs := map[string]bool{}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var entry handbackLine
		if json.Unmarshal(line, &entry) != nil {
			// One malformed line must not sink the whole scan — skip it.
			continue
		}

		beforeLaunched, beforeFinished := len(launchedIDs), len(finishedIDs)

		// The structured peer hand-back marker, read before the type switch
		// so it applies whichever entry type carries it (see the struct
		// comment above for the two real shapes).
		if entry.Origin != nil && entry.Origin.Kind == "peer" && entry.Origin.Handback && entry.Origin.From != "" && launchedIDs[entry.Origin.From] {
			finishedIDs[entry.Origin.From] = true
		}

		switch entry.Type {
		case "assistant":
			if entry.Message == nil {
				continue
			}
			for _, b := range handbackParseBlocks(entry.Message.Content) {
				if b.Type == "tool_use" && (b.Name == "Agent" || b.Name == "Task") {
					pendingLaunchToolUseIDs[b.ID] = true
				}
			}
		case "user":
			if entry.Message == nil {
				continue
			}
			if s, ok := handbackRawString(entry.Message.Content); ok {
				handbackScanText(s, launchedIDs, finishedIDs)
				continue
			}
			for _, b := range handbackParseBlocks(entry.Message.Content) {
				switch b.Type {
				case "tool_result":
					if !pendingLaunchToolUseIDs[b.ToolUseID] {
						continue
					}
					text := handbackBlockText(b.Content)
					if !strings.Contains(text, handbackAsyncLaunchMarker) {
						continue
					}
					id := ""
					if entry.ToolUseResult != nil && entry.ToolUseResult.AgentID != "" {
						id = entry.ToolUseResult.AgentID
					} else if m := handbackLaunchIDRE.FindStringSubmatch(text); m != nil {
						id = m[1]
					}
					if id != "" {
						launchedIDs[id] = true
					}
				case "text":
					handbackScanText(b.Text, launchedIDs, finishedIDs)
				}
			}
		case "attachment":
			if entry.Attachment == nil {
				continue
			}
			if entry.Attachment.Origin != nil && entry.Attachment.Origin.Handback && entry.Attachment.Origin.From != "" {
				if launchedIDs[entry.Attachment.Origin.From] {
					finishedIDs[entry.Attachment.Origin.From] = true
				}
			}
			handbackScanText(handbackBlockText(entry.Attachment.Prompt), launchedIDs, finishedIDs)
		}

		if len(launchedIDs) != beforeLaunched || len(finishedIDs) != beforeFinished {
			if ts, ok := parseHandbackTimestamp(entry.Timestamp); ok && (!hasProgress || ts.After(lastProgress)) {
				lastProgress = ts
				hasProgress = true
			}
		}
	}
	if scanErr := sc.Err(); scanErr != nil {
		return nil, nil, time.Time{}, false, scanErr
	}
	return launchedIDs, finishedIDs, lastProgress, hasProgress, nil
}

// handbackScanText marks a launched id finished when text carries either
// terminal marker for it: a hand-back or a task-notification. An id not in
// launchedIDs is never added — the text a report or notification sits inside
// is never itself proof that this transcript actually launched that id.
func handbackScanText(text string, launchedIDs, finishedIDs map[string]bool) {
	for _, m := range handbackReportRE.FindAllStringSubmatch(text, -1) {
		if launchedIDs[m[1]] {
			finishedIDs[m[1]] = true
		}
	}
	for _, m := range handbackTaskNotificationRE.FindAllStringSubmatch(text, -1) {
		if launchedIDs[m[1]] {
			finishedIDs[m[1]] = true
		}
	}
}

// handbackParseBlocks decodes a message.content value as a content-block
// array. A value that is not an array (e.g. a plain string) returns nil —
// callers try handbackRawString first for that shape.
func handbackParseBlocks(raw json.RawMessage) []handbackBlock {
	if len(raw) == 0 {
		return nil
	}
	var blocks []handbackBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return nil
	}
	return blocks
}

// handbackRawString decodes a JSON value as a plain string, reporting
// whether it was one. message.content and a tool_result's own content can
// each be either a bare string or an array of blocks.
func handbackRawString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// handbackBlockText extracts the text of a tool_result's own content value,
// whichever of the two shapes it takes.
func handbackBlockText(raw json.RawMessage) string {
	if s, ok := handbackRawString(raw); ok {
		return s
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var sb strings.Builder
	for _, b := range blocks {
		sb.WriteString(b.Text)
	}
	return sb.String()
}

// handbackDenyReason tells Hermes exactly what to do next, one instruction
// per sentence, then lists who it is still waiting on.
func handbackDenyReason(missing []string) string {
	base := "End your turn now. The harness wakes you when a child reports. Call SubagentHandback once, after the last child."
	if len(missing) == 0 {
		return base
	}
	return base + " Still missing: " + strings.Join(missing, ", ") + "."
}

// handbackStallContext is the visible trace the 30-minute stall release
// leaves behind. The hand-back proceeds (handbackGateDecision sets no
// Decision alongside it), but the missing children must be named in the
// report, not silently dropped.
func handbackStallContext(missing []string) string {
	return "Stall timeout reached: no launch or finish for " + handbackStallTimeout.String() +
		". Still missing: " + strings.Join(missing, ", ") +
		". You may hand back now. State in your report that these children's tiers are parent-only, not child-verified."
}
