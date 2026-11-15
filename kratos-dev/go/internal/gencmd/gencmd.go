package gencmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CommandFile is one generated commands/<name>.md target.
type CommandFile struct {
	Name    string
	Path    string
	Desired string
}

// Plan is the full set of desired outputs computed from agents/*.md +
// partials, without writing anything to disk.
type Plan struct {
	RepoRoot     string
	Agents       map[string]*Agent
	Commands     []CommandFile // sorted by Name
	SkillPath    string
	SkillDesired string
	ModesPath    string
	ModesDesired string
}

// BuildPlan loads agents + partials and renders every desired output.
func BuildPlan(repoRoot string) (*Plan, error) {
	agents, err := LoadAgents(repoRoot)
	if err != nil {
		return nil, err
	}
	partials, err := LoadPartials(repoRoot)
	if err != nil {
		return nil, err
	}

	commandsDir := filepath.Join(repoRoot, "plugins", "kratos", "commands")
	suffixDir := filepath.Join(repoRoot, "plugins", "kratos", "command-mode-suffix")

	names := SortedNames(agents)
	files := make([]CommandFile, 0, len(names))
	for _, name := range names {
		a := agents[name]
		hasSuffix := fileExists(filepath.Join(suffixDir, name+".md"))
		files = append(files, CommandFile{
			Name:    name,
			Path:    filepath.Join(commandsDir, name+".md"),
			Desired: RenderCommand(a, partials[name], hasSuffix),
		})
	}

	skillPath := filepath.Join(repoRoot, "plugins", "kratos", "skills", "auto", "SKILL.md")
	skillRaw, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", skillPath, err)
	}
	skillDesired, err := RenderSkillGodRegions(normalizeLF(string(skillRaw)), agents)
	if err != nil {
		return nil, err
	}

	modesPath := filepath.Join(repoRoot, "plugins", "kratos", "modes", "modes.md")
	modesRaw, err := os.ReadFile(modesPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", modesPath, err)
	}
	modesDesired, err := RenderModesModelMatrix(normalizeLF(string(modesRaw)), agents)
	if err != nil {
		return nil, err
	}

	return &Plan{
		RepoRoot:     repoRoot,
		Agents:       agents,
		Commands:     files,
		SkillPath:    skillPath,
		SkillDesired: skillDesired,
		ModesPath:    modesPath,
		ModesDesired: modesDesired,
	}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// writeFileAtomic writes data to a sibling ".tmp" file and renames it into
// place, so a mid-write kill (or a reader racing the write) never observes a
// truncated commands/<god>.md or SKILL.md. Rename is atomic on the same
// volume on both POSIX and Windows.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func readIfExists(path string) (*string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	s := string(raw)
	return &s, nil
}

func isGenerated(content string) bool {
	fm, err := ParseFrontmatter("", content)
	if err != nil {
		return false
	}
	return fm.Fields["generated"] == "true"
}

// Orphans returns generated commands/*.md files whose corresponding agent no
// longer exists in plugins/kratos/agents/. Only files carrying the
// `generated: true` marker are considered — hand-written non-god commands
// (main.md, quick.md, ...) never match.
func (p *Plan) Orphans() ([]string, error) {
	commandsDir := filepath.Join(p.RepoRoot, "plugins", "kratos", "commands")
	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		return nil, fmt.Errorf("reading commands dir %s: %w", commandsDir, err)
	}

	var orphans []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		if _, ok := p.Agents[name]; ok {
			continue
		}
		path := filepath.Join(commandsDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		if isGenerated(normalizeLF(string(raw))) {
			orphans = append(orphans, path)
		}
	}
	sort.Strings(orphans)
	return orphans, nil
}

// DriftResult names one file that differs from its desired generated
// content, or is missing/orphaned.
type DriftResult struct {
	Path   string
	Reason string // "missing" | "changed" | "orphan"
}

// Check renders everything in memory and diffs it against disk without
// writing. Returns one DriftResult per file that doesn't match.
func (p *Plan) Check() ([]DriftResult, error) {
	var drift []DriftResult

	for _, f := range p.Commands {
		cur, err := readIfExists(f.Path)
		if err != nil {
			return nil, err
		}
		if cur == nil {
			drift = append(drift, DriftResult{Path: f.Path, Reason: "missing"})
			continue
		}
		if normalizeLF(*cur) != f.Desired {
			drift = append(drift, DriftResult{Path: f.Path, Reason: "changed"})
		}
	}

	skillCur, err := os.ReadFile(p.SkillPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", p.SkillPath, err)
	}
	if normalizeLF(string(skillCur)) != p.SkillDesired {
		drift = append(drift, DriftResult{Path: p.SkillPath, Reason: "changed"})
	}

	modesCur, err := os.ReadFile(p.ModesPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", p.ModesPath, err)
	}
	if normalizeLF(string(modesCur)) != p.ModesDesired {
		drift = append(drift, DriftResult{Path: p.ModesPath, Reason: "changed"})
	}

	orphans, err := p.Orphans()
	if err != nil {
		return nil, err
	}
	for _, o := range orphans {
		drift = append(drift, DriftResult{Path: o, Reason: "orphan"})
	}

	return drift, nil
}

// ApplyResult summarizes one write action taken by Apply.
type ApplyResult struct {
	Path   string
	Action string // "written" | "unchanged" | "deleted-orphan"
}

// Apply writes the plan's desired content to disk. Existing commands/*.md
// files that lack the `generated: true` marker are refused (non-nil error)
// unless adopt is true — the escape hatch for the first run that hands
// hand-written launchers over to the generator. Orphaned generated files
// (agent deleted) are removed.
//
// results is a named return so a mid-loop failure still reports everything
// written before the error — callers (e.g. cmd/gencommands) should print
// results even when err != nil, for visibility into a partially-applied run.
func (p *Plan) Apply(adopt bool) (results []ApplyResult, err error) {
	for _, f := range p.Commands {
		var cur *string
		cur, err = readIfExists(f.Path)
		if err != nil {
			return results, err
		}
		if cur != nil {
			normalized := normalizeLF(*cur)
			if normalized == f.Desired {
				results = append(results, ApplyResult{Path: f.Path, Action: "unchanged"})
				continue
			}
			if !isGenerated(normalized) && !adopt {
				err = fmt.Errorf("refusing to overwrite %s: no 'generated: true' marker (pass --adopt for the first run)", f.Path)
				return results, err
			}
		}
		if err = writeFileAtomic(f.Path, []byte(f.Desired), 0o644); err != nil {
			err = fmt.Errorf("writing %s: %w", f.Path, err)
			return results, err
		}
		results = append(results, ApplyResult{Path: f.Path, Action: "written"})
	}

	var skillCur []byte
	skillCur, err = os.ReadFile(p.SkillPath)
	if err != nil {
		err = fmt.Errorf("reading %s: %w", p.SkillPath, err)
		return results, err
	}
	if normalizeLF(string(skillCur)) != p.SkillDesired {
		if err = writeFileAtomic(p.SkillPath, []byte(p.SkillDesired), 0o644); err != nil {
			err = fmt.Errorf("writing %s: %w", p.SkillPath, err)
			return results, err
		}
		results = append(results, ApplyResult{Path: p.SkillPath, Action: "written"})
	} else {
		results = append(results, ApplyResult{Path: p.SkillPath, Action: "unchanged"})
	}

	var modesCur []byte
	modesCur, err = os.ReadFile(p.ModesPath)
	if err != nil {
		err = fmt.Errorf("reading %s: %w", p.ModesPath, err)
		return results, err
	}
	if normalizeLF(string(modesCur)) != p.ModesDesired {
		if err = writeFileAtomic(p.ModesPath, []byte(p.ModesDesired), 0o644); err != nil {
			err = fmt.Errorf("writing %s: %w", p.ModesPath, err)
			return results, err
		}
		results = append(results, ApplyResult{Path: p.ModesPath, Action: "written"})
	} else {
		results = append(results, ApplyResult{Path: p.ModesPath, Action: "unchanged"})
	}

	var orphans []string
	orphans, err = p.Orphans()
	if err != nil {
		return results, err
	}
	for _, o := range orphans {
		if err = os.Remove(o); err != nil {
			err = fmt.Errorf("deleting orphan %s: %w", o, err)
			return results, err
		}
		results = append(results, ApplyResult{Path: o, Action: "deleted-orphan"})
	}

	return results, nil
}
