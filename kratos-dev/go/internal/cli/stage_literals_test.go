package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The pipeline was renumbered to stages 0-9 in 2026-07; docs that still say
// "Stage X/11" or carry 10-/11- stage keys send agents to stages that do not
// exist (the 2026-07-04 audit asked for this lint; recall.md still had it in
// 2026-09). Every plugin markdown file must be free of the legacy literals.
var legacyStageLiteralRE = regexp.MustCompile(`Stage \[?\w+\]?/11\b|\b(?:10-prd-alignment|11-review|8-code-review|9-implementation)\b|\bStage 1[01]\b`)

func TestNoLegacyStageLiteralsInPluginDocs(t *testing.T) {
	pluginRoot := filepath.Join("..", "..", "..", "..", "plugins", "kratos")
	if _, err := os.Stat(pluginRoot); err != nil {
		t.Skip("plugin tree not present")
	}
	var offenders []string
	err := filepath.WalkDir(pluginRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".claude" || name == ".agents" || name == "node_modules" || name == "bin" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for i, line := range strings.Split(string(body), "\n") {
			if legacyStageLiteralRE.MatchString(line) {
				rel, _ := filepath.Rel(pluginRoot, path)
				offenders = append(offenders, rel+":"+itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("legacy stage literals found (pipeline is stages 0-9):\n  %s", strings.Join(offenders, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
