package cli

import (
	"strings"
	"testing"
)

// Iris and Ares opt into the artifact-edit protocol section; the composed
// block they receive must carry it, and every god's protocol_sections list
// must resolve (an unknown slug is a build bug).
func TestArtifactEditProtocolComposed(t *testing.T) {
	for _, god := range []string{"iris", "ares"} {
		block, err := composeProtocolFor(god)
		if err != nil {
			t.Fatalf("composeProtocolFor(%s): %v", god, err)
		}
		if !strings.Contains(block, "Artifact Edits") {
			t.Errorf("%s protocol block lacks the Artifact Edits section:\n%s", god, block)
		}
		if !strings.Contains(block, "End every turn with visible text") {
			t.Errorf("%s protocol block lacks the visible-text boundary rule", god)
		}
	}
}

func TestAllAgentsProtocolSectionsResolve(t *testing.T) {
	entries, err := agentsFS.ReadDir("agents")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if _, err := composeProtocolFor(e.Name()); err != nil {
			t.Errorf("%s: %v", e.Name(), err)
		}
	}
}
