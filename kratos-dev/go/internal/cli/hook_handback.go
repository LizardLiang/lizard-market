package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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
// any launched child is still outstanding. It never allows — a permitted
// hand-back produces no decision and follows Claude Code's normal flow, the
// same contract hook_editgate.go's gate keeps.
//
// Counting is scoped to JSON structure, not raw substring search. Hermes
// itself, or one of its children, can Read or Grep this very file, this
// plan, or a fixture that quotes "Async agent launched successfully" or
// <agent-message from="…"> in plain prose — a raw text scan over the
// transcript would count that quoted text as a real launch or report and
// deny every hand-back forever. Scoping a launch to a tool_result whose
// tool_use_id answers a real Agent/Task tool_use, and scoping a report to a
// non-tool-result entry, keeps an unrelated Read/Grep result inert.

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
	if res.Decision == "" {
		return
	}
	data, err := json.Marshal(preToolUseOutput{
		HookSpecificOutput: preToolUseHookSpecific{
			HookEventName:            "PreToolUse",
			PermissionDecision:       res.Decision,
			PermissionDecisionReason: res.Reason,
		},
	})
	if err != nil {
		return
	}
	fmt.Println(string(data))
}

// handbackGateResult is one gate verdict. Decision "" means no output at all.
type handbackGateResult struct {
	Decision string // "" (no decision) or "deny"
	Reason   string
}

// handbackGateDecision is the whole policy, as a pure function of the
// payload, so the table test can exercise every branch without touching a
// real transcript for the "no decision" cases.
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

	launchedIDs, finishedIDs, err := scanHandbackTranscript(path)
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

	if handbackBoundReached(input.Cwd, input.AgentID, len(finishedIDs)) {
		debugLog("handback-gate: bound reached for agent %s, allowing hand-back with %d still missing", input.AgentID, len(missing))
		return handbackGateResult{}
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
	Content   json.RawMessage `json:"content"`      // tool_result's own nested content
}

// handbackLine is the subset of one transcript JSONL entry this gate reads.
type handbackLine struct {
	Type    string `json:"type"`
	Message *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Attachment *struct {
		Prompt string `json:"prompt"`
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

// scanHandbackTranscript walks Hermes's own subagent transcript once and
// returns which children it launched and which of those have finished —
// either by reporting (<agent-message from="…">) or by a terminal
// <task-notification> naming their id, whatever that notification's status.
// A finish is only counted for an id this same transcript actually launched;
// an unrelated id appearing in some other message never counts.
func scanHandbackTranscript(path string) (launchedIDs, finishedIDs map[string]bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
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
			handbackScanText(entry.Attachment.Prompt, launchedIDs, finishedIDs)
		}
	}
	if scanErr := sc.Err(); scanErr != nil {
		return nil, nil, scanErr
	}
	return launchedIDs, finishedIDs, nil
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

// handbackBlockStatePath resolves this gate's own bounded-denial state file.
// There is no "feature" concept for a PreToolUse hook, so — like Hermes's own
// checklist fallback — it always lives under cwd's .claude/tmp/.
func handbackBlockStatePath(cwd string) string {
	return gateStatePath(cwd, "", "hermes-handback-gate.json")
}

// handbackBoundReached reports whether Hermes has already been denied
// gateMaxBlocks times in a row with no child finishing in between — the same
// cap v2.112.0 gave every other content gate, so this one cannot force
// continuations indefinitely either.
//
// A review whose children are simply slow must never trip this: any denial
// whose finished count is higher than the one recorded at the last denial is
// progress, and progress resets the counter to 1. finished is the count at
// the moment of THIS decision, from the same scan that produced it.
//
// A state read failure resets to a fresh count (the same fail-open behavior
// every other block-count guard in this file uses — see readGateBlockState).
// A state WRITE failure is the one place this gate fails in the opposite
// direction: if the counter cannot be persisted, every future call would
// read a stale count and this gate would deny forever, so an unwritable
// state file is itself treated as "bound reached."
func handbackBoundReached(cwd, agentID string, finished int) bool {
	path := handbackBlockStatePath(cwd)
	state := readGateBlockState(path)
	if agentID != "" && state.AgentID != "" && state.AgentID != agentID {
		state = gateBlockState{}
	}
	if finished > state.Progress {
		state.BlockCount = 0
		state.Progress = finished
	}
	if state.BlockCount >= gateMaxBlocks {
		return true
	}
	state.AgentID = agentID
	state.BlockCount++
	if err := writeHandbackBlockState(path, state); err != nil {
		debugLog("handback-gate: bound state write failed for %s, failing toward bound reached: %v", path, err)
		return true
	}
	return false
}

// writeHandbackBlockState persists this gate's bounded-denial state and
// reports failure, unlike writeGateBlockState's best-effort, error-swallowing
// write — see handbackBoundReached for why this one write must be checked.
func writeHandbackBlockState(path string, state gateBlockState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
