package cli

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useStateDir isolates HOME, writes cfg (when non-empty) as
// ~/.kratos/config.json, sets env (when non-empty) and clears the cache.
func useStateDir(t *testing.T, cfg, env string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KRATOS_STATE_DIR", env)
	if cfg != "" {
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".kratos"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".kratos", "config.json"), []byte(cfg), 0o644))
	}
	resetStateDirForTest()
	t.Cleanup(resetStateDirForTest)
	return home
}

func TestStateDirPrecedence(t *testing.T) {
	cases := []struct {
		name, cfg, env, want string
	}{
		{"default", "", "", ".claude"},
		{"config", `{"state_dir":".agents"}`, "", ".agents"},
		{"env beats config", `{"state_dir":".agents"}`, ".kratos", ".kratos"},
		{"absolute rejected", `{"state_dir":"/abs"}`, "", ".claude"},
		{"nested rejected", `{"state_dir":".a/b"}`, "", ".claude"},
		{"no dot rejected", `{"state_dir":"agents"}`, "", ".claude"},
		{"dotdot rejected", `{"state_dir":".."}`, "", ".claude"},
		{"bare dot rejected", `{"state_dir":"."}`, "", ".claude"},
		{"bad env rejected", "", "x/y", ".claude"},
		{"corrupt config ignored", `{not json`, "", ".claude"},
		{"empty value ignored", `{"state_dir":""}`, "", ".claude"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			useStateDir(t, c.cfg, c.env)
			assert.Equal(t, c.want, stateDirName())
		})
	}
}

func TestStatePathsFollowSetting(t *testing.T) {
	useStateDir(t, `{"state_dir":".agents"}`, "")
	root := filepath.Join("r", "p")
	assert.Equal(t, filepath.Join(root, ".agents", "feature"), featureRoot(root))
	assert.Equal(t, filepath.Join(root, ".agents", ".Arena"), arenaRoot(root))
	assert.Equal(t, filepath.Join(root, ".agents", "tmp"), tmpRoot(root))
	assert.Equal(t, ".agents/.Arena/tactical-plans", gatePlanRoot())
}

func TestRewriteStateText(t *testing.T) {
	in := "a .claude/feature/x, .claude/.Arena/y, .claude/.arena/z, .claude/tmp/w, ~/.claude/settings.json, .claude/projects"
	useStateDir(t, "", "")
	assert.Equal(t, in, rewriteStateText(in))
	useStateDir(t, `{"state_dir":".agents"}`, "")
	assert.Equal(t, "a .agents/feature/x, .agents/.Arena/y, .agents/.arena/z, .agents/tmp/w, ~/.claude/settings.json, .claude/projects", rewriteStateText(in))
}

func TestAgentLoadRewriteAndDefaultByteEquality(t *testing.T) {
	entries, err := agentsFS.ReadDir("agents")
	require.NoError(t, err)

	// Version B: setting .agents, no god keeps a Kratos-owned .claude path.
	useStateDir(t, `{"state_dir":".agents"}`, "")
	for _, e := range entries {
		god := strings.TrimSuffix(e.Name(), ".md")
		out := runAgentLoad(t, god, "--resolve", "--root", "/r")
		for _, old := range []string{".claude/feature", ".claude/.Arena", ".claude/.arena", ".claude/tmp"} {
			assert.NotContains(t, out, old, "%s still names %s", god, old)
		}
	}

	// Version A: no setting, resolve changes tokens only.
	useStateDir(t, "", "")
	exe, err := os.Executable()
	require.NoError(t, err)
	for _, e := range entries {
		god := strings.TrimSuffix(e.Name(), ".md")
		raw := runAgentLoad(t, god)
		want := strings.ReplaceAll(raw, "<KRATOS_ROOT>", "/r")
		want = strings.ReplaceAll(want, "<kratos-bin>", toSlash(exe))
		assert.Equal(t, want, runAgentLoad(t, god, "--resolve", "--root", "/r"), "%s differs from the token-only output", god)
	}
}

func TestEditGateFollowsStateDir(t *testing.T) {
	cwd := filepath.ToSlash(filepath.Join(t.TempDir(), "proj"))
	spec := cwd + "/.agents/feature/x/spec-delta/y.md"
	oldSpec := cwd + "/.claude/feature/x/spec-delta/y.md"
	flow := cwd + "/.agents/.Arena/flows/s.md"
	oldFlow := cwd + "/.claude/.Arena/flows/s.md"

	useStateDir(t, `{"state_dir":".agents"}`, "")
	assert.True(t, isSpecDeltaGatePath(spec, cwd))
	assert.False(t, isSpecDeltaGatePath(oldSpec, cwd))
	assert.True(t, isFlowShardPath(flow, cwd))
	assert.False(t, isFlowShardPath(oldFlow, cwd))
	assert.False(t, isGateProjectFile(cwd+"/.agents/feature/x/prd.md", cwd))
	assert.True(t, isGateProjectFile(cwd+"/src/a.go", cwd))

	useStateDir(t, "", "")
	assert.True(t, isSpecDeltaGatePath(oldSpec, cwd))
	assert.False(t, isSpecDeltaGatePath(spec, cwd))
	assert.False(t, isGateProjectFile(cwd+"/.claude/feature/x/prd.md", cwd))
	assert.True(t, isGateProjectFile(cwd+"/.agents/a.go", cwd), "default setting does not exempt .agents")
}

func TestSpecDeltaPathFollowsStateDir(t *testing.T) {
	useStateDir(t, `{"state_dir":".agents"}`, "")
	m := specDeltaPathRE().FindStringSubmatch(`C:\p\.agents\feature\f1\spec-delta\c.md`)
	require.Len(t, m, 2)
	assert.Equal(t, "f1", m[1])
	assert.Nil(t, specDeltaPathRE().FindStringSubmatch(`C:\p\.claude\feature\f1\spec-delta\c.md`))
}

func TestVerifyLandedIgnoresStateDir(t *testing.T) {
	for _, tc := range []struct{ cfg, dir string }{{`{"state_dir":".agents"}`, ".agents"}, {"", ".claude"}} {
		useStateDir(t, tc.cfg, "")
		dir, hash := gitRepo(t)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, tc.dir, "feature", "x"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, tc.dir, "feature", "x", "status.json"), []byte("{}"), 0o644))
		assert.Empty(t, evaluateLanded(dir, hash), "%s is Kratos state", tc.dir)
	}
	useStateDir(t, "", "")
	dir, hash := gitRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".agents", "a.md"), []byte("x"), 0o644))
	assert.NotEmpty(t, evaluateLanded(dir, hash), ".agents counts as work when it is not the state dir")
}

func TestPipelineInitUsesStateDir(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(wd) })
	for _, tc := range []struct{ cfg, dir string }{{`{"state_dir":".agents"}`, ".agents"}, {"", ".claude"}} {
		useStateDir(t, tc.cfg, "")
		dir, _ := gitRepo(t)
		require.NoError(t, os.Chdir(dir))
		cmd := PipelineCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"init", "--feature", "demo", "--description", "d"})
		require.NoError(t, cmd.Execute(), out.String())
		_, err := os.Stat(filepath.Join(dir, tc.dir, "feature", "demo", "status.json"))
		assert.NoError(t, err, out.String())
		require.NoError(t, os.Chdir(wd))
	}
}

// TestNoHardcodedKratosStatePaths fails when non-test code in this package
// names a Kratos-owned .claude path outside statedir.go. A literal is fine
// when it is an argument of rewriteStateText.
func TestNoHardcodedKratosStatePaths(t *testing.T) {
	banned := []string{".claude/feature", ".claude/.Arena", ".claude/.arena", ".claude/tmp", `\.claude/feature`, `\.claude/\.Arena`, `\.claude[/`}
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "statedir.go" {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		require.NoError(t, err)
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, _ := strconv.Unquote(lit.Value)
			for _, b := range banned {
				if !strings.Contains(val, b) {
					continue
				}
				wrapped := false
				for _, anc := range stack {
					if call, ok := anc.(*ast.CallExpr); ok {
						if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "rewriteStateText" {
							wrapped = true
						}
					}
				}
				if !wrapped {
					t.Errorf("%s: literal names %q; use the statedir.go helpers", fset.Position(lit.Pos()), b)
				}
			}
			if val == ".claude" && len(stack) >= 2 {
				if call, ok := stack[len(stack)-2].(*ast.CallExpr); ok {
					for i, a := range call.Args {
						if a != ast.Expr(lit) || i+1 >= len(call.Args) {
							continue
						}
						if nx, ok := call.Args[i+1].(*ast.BasicLit); ok {
							switch nx.Value {
							case `"feature"`, `".Arena"`, `"tmp"`:
								t.Errorf("%s: split .claude join; use the statedir.go helpers", fset.Position(lit.Pos()))
							}
						}
					}
				}
			}
			return true
		})
	}
}
