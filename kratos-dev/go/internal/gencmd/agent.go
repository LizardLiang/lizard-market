package gencmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LizardLiang/lizard-market/plugins/kratos/internal/protocol"
)

// Agent captures the generator-relevant frontmatter fields of a
// plugins/kratos/agents/<name>.md file. Name/description/model/model_eco/
// model_power are required; the rest are optional — absence means the
// launcher uses the boilerplate default.
type Agent struct {
	Name        string
	Description string
	Stage       string // "" if the god has no pipeline stage suffix
	QuickRoute  bool
	CommandRefs string // "" means default (equivalent to "standard")
	CommandNote string // "" means no override of the standard persona clause
	Model       string // normal-mode alias: haiku|sonnet|opus
	ModelEco    string // eco-mode alias: haiku|sonnet|opus
	ModelPower  string // power-mode alias: haiku|sonnet|opus

	// ProtocolSections lists the agent-protocol.md section slugs injected
	// into this agent at spawn/load time; validated against the anchors in
	// references/agent-protocol.md so slug drift fails make gen / gen-check.
	ProtocolSections []string
}

// validModelAliases is the single source of truth for the model routing
// enum — every agents/*.md model/model_eco/model_power field must be exactly
// one of these. Pinned model IDs (e.g. claude-sonnet-4-6) are rejected here
// and separately banned repo-wide by TestNoPinnedModelIDs.
var validModelAliases = map[string]bool{"haiku": true, "sonnet": true, "opus": true}

// LoadAgents reads every plugins/kratos/agents/*.md file under repoRoot and
// parses the generator-relevant frontmatter fields, keyed by agent name. It
// fails loud (non-nil error naming the file and field) on a missing
// name/description or unparseable frontmatter.
func LoadAgents(repoRoot string) (map[string]*Agent, error) {
	dir := filepath.Join(repoRoot, "plugins", "kratos", "agents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading agents dir %s: %w", dir, err)
	}

	// Protocol doc for slug validation — loaded lazily on the first agent
	// that declares protocol_sections, so fixture trees without the
	// references file stay loadable as long as no agent needs it.
	var protoDoc *protocol.Doc
	loadProtoDoc := func() (*protocol.Doc, error) {
		if protoDoc != nil {
			return protoDoc, nil
		}
		p := filepath.Join(repoRoot, "plugins", "kratos", "references", "agent-protocol.md")
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		protoDoc, err = protocol.Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		return protoDoc, nil
	}

	agents := map[string]*Agent{}
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

		name := strings.TrimSpace(fm.Fields["name"])
		if name == "" {
			return nil, fmt.Errorf("%s: missing required frontmatter field 'name'", path)
		}
		desc := strings.TrimSpace(fm.Fields["description"])
		if desc == "" {
			return nil, fmt.Errorf("%s: missing required frontmatter field 'description'", path)
		}

		expectedName := strings.TrimSuffix(e.Name(), ".md")
		if name != expectedName {
			return nil, fmt.Errorf("%s: frontmatter name %q does not match filename %q", path, name, expectedName)
		}

		refs := fm.Fields["command_refs"]
		if refs != "" {
			if _, ok := commandRefsSentences[refs]; !ok {
				return nil, fmt.Errorf("%s: invalid command_refs %q (want one of %s)", path, refs, strings.Join(sortedCommandRefsKeys(), "|"))
			}
		}

		model, err := requireModelAlias(path, fm.Fields, "model")
		if err != nil {
			return nil, err
		}
		modelEco, err := requireModelAlias(path, fm.Fields, "model_eco")
		if err != nil {
			return nil, err
		}
		modelPower, err := requireModelAlias(path, fm.Fields, "model_power")
		if err != nil {
			return nil, err
		}

		sections := protocol.ParseList(fm.Fields["protocol_sections"])
		if len(sections) > 0 {
			doc, err := loadProtoDoc()
			if err != nil {
				return nil, err
			}
			if _, err := protocol.Compose(doc, sections); err != nil {
				return nil, fmt.Errorf("%s: protocol_sections: %w", path, err)
			}
		}

		agents[name] = &Agent{
			Name:        name,
			Description: fm.Fields["description"],
			Stage:       fm.Fields["stage"],
			QuickRoute:  fm.Fields["quick_route"] == "true",
			CommandRefs: refs,
			CommandNote: fm.Fields["command_note"],
			Model:       model,
			ModelEco:    modelEco,
			ModelPower:  modelPower,

			ProtocolSections: sections,
		}
	}
	return agents, nil
}

// requireModelAlias reads field from fm, failing loud (naming path and
// field) if it's absent or not one of the haiku|sonnet|opus aliases.
func requireModelAlias(path string, fm map[string]string, field string) (string, error) {
	v := strings.TrimSpace(fm[field])
	if v == "" {
		return "", fmt.Errorf("%s: missing required frontmatter field %q", path, field)
	}
	if !validModelAliases[v] {
		return "", fmt.Errorf("%s: invalid %s %q (want one of haiku|sonnet|opus)", path, field, v)
	}
	return v, nil
}

// SortedNames returns agent names sorted alphabetically for deterministic
// (byte-stable) iteration order over the map.
func SortedNames(agents map[string]*Agent) []string {
	names := make([]string, 0, len(agents))
	for n := range agents {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
