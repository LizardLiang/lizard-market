package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
// This gate reads Hermes's own subagent transcript, counts the two markers,
// and denies the hand-back while children are still outstanding. It never
// allows — a permitted hand-back produces no decision and follows Claude
// Code's normal flow, the same contract hook_editgate.go's gate keeps.

// handbackAsyncLaunchMarker is the literal text the harness returns as a tool
// result when a subagent spawns a child asynchronously.
const handbackAsyncLaunchMarker = "Async agent launched successfully"

// handbackLaunchIDRE pairs a launch marker with the child's agent id, printed
// on the same transcript line as "agentId: <id>".
var handbackLaunchIDRE = regexp.MustCompile(`agentId:\s*([A-Za-z0-9_-]+)`)

// handbackReportRE matches a child's hand-back arriving in the parent's own
// transcript: <agent-message from="<agentId>">. The quote is optionally
// backslash-escaped because the transcript line is the tool's raw JSON
// encoding, where a literal `"` inside a string value is written `\"`.
var handbackReportRE = regexp.MustCompile(`<agent-message from=\\?"([^"\\]+)\\?">`)

func handbackGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "handback-gate",
		Short: "Handle PreToolUse for SubagentHandback — deny Hermes's hand-back until every launched child has reported",
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

	launched, launchedIDs, reportedIDs, err := scanHandbackTranscript(path)
	if err != nil {
		debugLog("handback-gate: transcript read error for %s: %v", path, err)
		return handbackGateResult{}
	}
	if launched <= len(reportedIDs) {
		return handbackGateResult{}
	}
	return handbackGateResult{Decision: "deny", Reason: handbackDenyReason(launched, reportedIDs, launchedIDs)}
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

// scanHandbackTranscript counts how many children Hermes launched
// asynchronously and which have reported back. launched is a raw count of
// the async-launch marker; the deny decision rests on that count against
// len(reportedIDs). launchedIDs is a best-effort id extraction, used only to
// name the gap in the deny reason.
func scanHandbackTranscript(path string) (launched int, launchedIDs []string, reportedIDs map[string]bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, nil, nil, err
	}
	defer f.Close()

	reportedIDs = map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		launched += strings.Count(line, handbackAsyncLaunchMarker)
		for _, m := range handbackLaunchIDRE.FindAllStringSubmatch(line, -1) {
			launchedIDs = append(launchedIDs, m[1])
		}
		for _, m := range handbackReportRE.FindAllStringSubmatch(line, -1) {
			reportedIDs[m[1]] = true
		}
	}
	if scanErr := sc.Err(); scanErr != nil {
		return 0, nil, nil, scanErr
	}
	return launched, launchedIDs, reportedIDs, nil
}

// handbackDenyReason names the children that have not reported yet, when
// their ids were recoverable from the transcript, and falls back to a plain
// count when they were not.
func handbackDenyReason(launched int, reportedIDs map[string]bool, launchedIDs []string) string {
	seen := map[string]bool{}
	var missing []string
	for _, id := range launchedIDs {
		if reportedIDs[id] || seen[id] {
			continue
		}
		seen[id] = true
		missing = append(missing, id)
	}

	base := fmt.Sprintf(
		"Hermes launched %d review agent(s). Only %d reported. Wait for the rest, then call SubagentHandback once.",
		launched, len(reportedIDs),
	)
	if len(missing) == 0 {
		return base
	}
	return base + " Still missing: " + strings.Join(missing, ", ") + "."
}
