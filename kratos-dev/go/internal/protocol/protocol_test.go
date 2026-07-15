package protocol

import (
	"os"
	"strings"
	"testing"
)

const sample = `# Agent Protocol — Shared Procedures

Preamble.

---

## Alpha Section
<!-- protocol: alpha -->

Alpha body line.

---

## Beta Section (extra)
<!-- protocol: beta -->

Beta body.

` + "```bash\nbeta code\n```" + `

---

## Gamma
<!-- protocol: gamma -->

Gamma body.
`

func TestParseSample(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(doc.Order, ","); got != "alpha,beta,gamma" {
		t.Fatalf("order = %q", got)
	}
	if !strings.HasPrefix(doc.Sections["alpha"], "## Alpha Section") {
		t.Errorf("alpha missing heading: %q", doc.Sections["alpha"])
	}
	if strings.Contains(doc.Sections["alpha"], "<!-- protocol:") {
		t.Errorf("anchor not stripped: %q", doc.Sections["alpha"])
	}
	if strings.Contains(doc.Sections["alpha"], "---") {
		t.Errorf("separator not stripped: %q", doc.Sections["alpha"])
	}
	if !strings.Contains(doc.Sections["beta"], "beta code") {
		t.Errorf("beta lost code block: %q", doc.Sections["beta"])
	}
}

func TestParseCRLFIdenticalToLF(t *testing.T) {
	lf, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	crlf, err := Parse(strings.ReplaceAll(sample, "\n", "\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range lf.Order {
		if lf.Sections[s] != crlf.Sections[s] {
			t.Errorf("section %q differs between LF and CRLF parse", s)
		}
	}
}

func TestParseMissingAnchor(t *testing.T) {
	if _, err := Parse("## No Anchor\n\nBody.\n"); err == nil {
		t.Fatal("expected error for heading without anchor")
	}
}

func TestParseDuplicateAnchor(t *testing.T) {
	md := "## A\n<!-- protocol: dup -->\n\nx\n\n## B\n<!-- protocol: dup -->\n\ny\n"
	if _, err := Parse(md); err == nil {
		t.Fatal("expected error for duplicate anchor")
	}
}

func TestCompose(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	// List order gamma-first; output must follow document order.
	out, err := Compose(doc, []string{"gamma", "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, Header) {
		t.Errorf("missing header:\n%s", out)
	}
	ai, gi := strings.Index(out, "## Alpha Section"), strings.Index(out, "## Gamma")
	if ai < 0 || gi < 0 || ai > gi {
		t.Errorf("document order not preserved (alpha=%d gamma=%d)", ai, gi)
	}
	if strings.Contains(out, "Beta") {
		t.Errorf("unselected section leaked:\n%s", out)
	}
}

func TestComposeErrors(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Compose(doc, []string{"nope"}); err == nil {
		t.Error("expected unknown-slug error")
	}
	if _, err := Compose(doc, []string{"alpha", "alpha"}); err == nil {
		t.Error("expected duplicate-slug error")
	}
	if out, err := Compose(doc, nil); err != nil || out != "" {
		t.Errorf("empty list should compose to empty string, got %q, %v", out, err)
	}
}

func TestParseList(t *testing.T) {
	got := ParseList(" Alpha, beta ,, gamma ")
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
	if ParseList("") != nil {
		t.Error("empty input should return nil")
	}
}

// TestParseRealFile parses the plugin's actual agent-protocol.md and asserts
// the full slug registry in document order.
func TestParseRealFile(t *testing.T) {
	raw, err := os.ReadFile("../../../../plugins/kratos/references/agent-protocol.md")
	if err != nil {
		t.Skipf("plugin source tree not available: %v", err)
	}
	doc, err := Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"path-resolution", "document-selection", "auto-discovery",
		"missing-required-input", "interactive-questions", "spawn-prompt-fields",
		"document-creation", "timestamp-standard", "status-updates",
		"spawning-athena", "session-tracking", "boundaries", "output-format",
	}
	if got := strings.Join(doc.Order, ","); got != strings.Join(want, ",") {
		t.Fatalf("section registry drift:\n got %s\nwant %s", got, strings.Join(want, ","))
	}
}
