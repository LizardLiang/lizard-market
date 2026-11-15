package gencmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Partial represents a bespoke launcher tail loaded from
// kratos-dev/codegen/partials/<god>.md. Keyed by god name (the filename minus
// .md); only ares/hera/prometheus have one today, but any god may gain one.
type Partial struct {
	Placement    string // "before-request" (default) or "after-request"
	AllowedTools string // "" if the partial doesn't extend allowed-tools
	Body         string // raw markdown tail, frontmatter stripped
}

// LoadPartials reads every kratos-dev/codegen/partials/*.md file under
// repoRoot. A missing partials directory is not an error (yields none).
func LoadPartials(repoRoot string) (map[string]*Partial, error) {
	dir := filepath.Join(repoRoot, "kratos-dev", "codegen", "partials")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*Partial{}, nil
		}
		return nil, fmt.Errorf("reading partials dir %s: %w", dir, err)
	}

	partials := map[string]*Partial{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		fm, err := ParseFrontmatter(path, normalizeLF(string(raw)))
		if err != nil {
			return nil, err
		}

		placement := fm.Fields["placement"]
		if placement == "" {
			placement = "before-request"
		}
		if placement != "before-request" && placement != "after-request" {
			return nil, fmt.Errorf("%s: invalid placement %q (want before-request|after-request)", path, placement)
		}

		god := strings.TrimSuffix(e.Name(), ".md")
		partials[god] = &Partial{
			Placement:    placement,
			AllowedTools: fm.Fields["allowed-tools"],
			Body:         strings.Trim(fm.Body, "\n"),
		}
	}
	return partials, nil
}
