package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runTemplate(t *testing.T, args ...string) string {
	t.Helper()
	cmd := TemplateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return out.String()
}

func TestTemplateGetRewritesStateDir(t *testing.T) {
	useStateDir(t, "", ".agents")
	got := runTemplate(t, "get", "ares-plan-mission")
	assert.Contains(t, got, ".agents/feature")
	assert.NotContains(t, got, ".claude/feature")
}

func TestTemplateGetDefaultIsEmbeddedBytes(t *testing.T) {
	useStateDir(t, "", "")
	want, err := templatesFS.ReadFile("templates/ares-plan-mission.md")
	require.NoError(t, err)
	assert.Equal(t, string(want), runTemplate(t, "get", "ares-plan-mission"))
}

func TestTemplateCopyRewritesStateDir(t *testing.T) {
	useStateDir(t, "", ".agents")
	dest := filepath.Join(t.TempDir(), "out", "mission.md")
	runTemplate(t, "copy", "ares-plan-mission", dest)
	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Contains(t, string(data), ".agents/feature")
	assert.NotContains(t, string(data), ".claude/feature")
}

func TestTemplateCopyHTMLRewritesStateDir(t *testing.T) {
	useStateDir(t, "", ".agents")
	dest := filepath.Join(t.TempDir(), "plan.html")
	runTemplate(t, "copy", "plan-page-template", dest)
	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Contains(t, string(data), ".agents/.Arena")
	assert.NotContains(t, string(data), ".claude/.Arena")
}

func TestTemplateCopyHTMLDefaultIsEmbeddedBytes(t *testing.T) {
	useStateDir(t, "", "")
	want, err := templatesFS.ReadFile("templates/plan-page-template.html")
	require.NoError(t, err)
	dest := filepath.Join(t.TempDir(), "plan.html")
	runTemplate(t, "copy", "plan-page-template", dest)
	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, want, data)
}
