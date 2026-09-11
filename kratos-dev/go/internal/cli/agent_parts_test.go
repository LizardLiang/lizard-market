package cli

import (
	"io"
	"strings"
	"testing"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// `agent load --part body|extras` splits a definition into two slices that
// each fit Claude Code's 30,000-char inline limit for a !`cmd` line (a whole
// god is 31–34 KB and reached the model as a 2 KB <persisted-output> preview
// in the 2026-09 review). body + extras is the legacy single-stream output.
func TestAgentLoad_Parts(t *testing.T) {
	useTempDB(t)
	conn, err := db.GetConnection()
	require.NoError(t, err)
	defer conn.Close()
	_, err = db.AddFeedback(conn, "odysseus", "Ask one clarity question at a time; he rejects batches.", "some-project")
	require.NoError(t, err)

	body := runAgentLoad(t, "odysseus", "--part", "body")
	assert.Contains(t, body, "# Odysseus - King of Ithaca")
	assert.NotContains(t, body, "# Agent Protocol (injected)")
	assert.NotContains(t, body, "Lessons from past user corrections")

	extras := runAgentLoad(t, "odysseus", "--part", "extras")
	assert.NotContains(t, extras, "# Odysseus - King of Ithaca")
	lessonsAt := strings.Index(extras, "Lessons from past user corrections of odysseus")
	protocolAt := strings.Index(extras, "# Agent Protocol (injected)")
	require.GreaterOrEqual(t, lessonsAt, 0, "extras carries the lessons block")
	require.GreaterOrEqual(t, protocolAt, 0, "extras carries the protocol block")
	assert.Less(t, lessonsAt, protocolAt, "lessons come first so a partial read still sees them")
	assert.Contains(t, extras, "Ask one clarity question at a time")

	full := runAgentLoad(t, "odysseus")
	assert.Equal(t, body+"\n---\n\n"+extras, full, "body + extras must equal the legacy full output")

	// Command mode: the suffix rides in extras, after the protocol; the body
	// never carries it.
	suffix, err := commandSuffixFS.ReadFile("command-mode-suffix/hermes.md")
	require.NoError(t, err)
	marker := ""
	for _, line := range strings.Split(string(suffix), "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "#") {
			marker = line // first heading of the suffix, unique to it
			break
		}
	}
	require.NotEmpty(t, marker, "hermes command-mode suffix has no heading to anchor on")
	hb := runAgentLoad(t, "hermes", "--part", "body", "--mode=command")
	assert.NotContains(t, hb, marker)
	he := runAgentLoad(t, "hermes", "--part", "extras", "--mode=command")
	suffixAt := strings.Index(he, marker)
	hProtocolAt := strings.Index(he, "# Agent Protocol (injected)")
	require.GreaterOrEqual(t, suffixAt, 0, "command-mode suffix lands in extras")
	assert.Less(t, hProtocolAt, suffixAt, "protocol before suffix")

	// No lessons, no suffix: extras is just the protocol block, no leading
	// separator.
	ce := runAgentLoad(t, "clio", "--part", "extras")
	assert.True(t, strings.HasPrefix(ce, "# Agent Protocol (injected)"), "clio extras should start with the protocol header, got:\n%s", truncate(ce))
}

func TestAgentLoad_PartRejectsUnknown(t *testing.T) {
	cmd := AgentCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"load", "clio", "--part", "foo"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --part")
}
