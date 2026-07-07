package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStderr redirects os.Stderr to a buffer for the duration of fn, mirroring
// check_test.go's captureStdout for the stream loadSpecShardsIn's skipped-shard
// warning writes to.
func captureStderr(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// ---------- fixtures ----------

const exportShardAuth = `---
created: 2026-01-01T00:00:00Z
updated: 2026-01-02T00:00:00Z
author: metis
git_hash: abc123
capability: auth-system
---

# Auth System

## Purpose

Handles authentication & authorization for the <platform>.

## Requirements

### Requirement: Password Login

The system SHALL allow users to log in with a password.

#### Scenario: valid credentials

- **WHEN** a user submits correct credentials
- **THEN** the system SHALL grant access
- **AND** the system SHALL log the event

### Requirement: Rate Limiting

The system SHALL rate-limit login attempts using ` + "`429 Too Many Requests`" + `.

Some unstructured notes:

| Attempt | Limit |
| --- | --- |
| 1 | 5/min |

#### Scenario: too many attempts

- **WHEN** a user exceeds 5 attempts in a minute
- **THEN** the system SHALL respond with 429
`

const exportShardBilling = `---
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
author: metis
git_hash: def456
capability: billing
---

# Billing

## Purpose

Handles invoicing.

## Requirements

### Requirement: Invoice Generation

The system SHALL generate an invoice at the end of each billing cycle.

#### Scenario: cycle ends

- **WHEN** a billing cycle ends
- **THEN** the system SHALL generate an invoice
`

// normalizeNewlines first guards this fixture against the source file's own
// on-disk line-ending style (e.g. a Windows checkout with core.autocrlf=true) —
// without it, an already-CRLF source file would double up into "\r\r\n".
var exportShardBillingCRLF = strings.ReplaceAll(normalizeNewlines(exportShardBilling), "\n", "\r\n")

const exportShardXSS = `---
created: 2026-01-01T00:00:00Z
updated: 2026-01-01T00:00:00Z
author: metis
git_hash: xss000
capability: xss-check
---

# XSS Check

## Purpose

Untrusted content handling.

## Requirements

### Requirement: Sanitized Rendering

The system SHALL sanitize <script>alert(1)</script> & similar constructs before display.

#### Scenario: hostile input

- **WHEN** a requirement body contains markup-like text
- **THEN** the system SHALL render it as literal escaped text
`

// ---------- renderer unit tests ----------

func TestRenderInlineEscaping(t *testing.T) {
	got := renderInline("a & b <c> `d` **e**")
	want := "a &amp; b &lt;c&gt; <code>d</code> <strong>e</strong>"
	if got != want {
		t.Errorf("renderInline() = %q, want %q", got, want)
	}
}

func TestRenderBlockHTMLScenarioAndBullets(t *testing.T) {
	input := "The system SHALL do X.\n\n#### Scenario: happy path\n\n" +
		"- **WHEN** something happens\n- **THEN** the system SHALL respond\n- **AND** it SHALL log the event\n"
	got := renderBlockHTML(normalizeNewlines(input))
	want := "<p>The system SHALL do X.</p>\n" +
		"<h4 class=\"scenario\">Scenario: happy path</h4>\n" +
		"<ul>\n" +
		"<li><strong class=\"kw\">WHEN</strong> something happens</li>\n" +
		"<li><strong class=\"kw\">THEN</strong> the system SHALL respond</li>\n" +
		"<li><strong class=\"kw\">AND</strong> it SHALL log the event</li>\n" +
		"</ul>\n"
	if got != want {
		t.Errorf("renderBlockHTML() =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderBlockHTMLUnknownConstructFallback(t *testing.T) {
	input := "| Attempt | Limit |\n| --- | --- |\n| 1 | 5/min |"
	got := renderBlockHTML(input)
	want := "<p>| Attempt | Limit | | --- | --- | | 1 | 5/min |</p>\n"
	if got != want {
		t.Errorf("renderBlockHTML(table) = %q, want %q", got, want)
	}
}

func TestSlugifyHeading(t *testing.T) {
	got := slugifyHeading("Requirement: Password Login")
	want := "requirement-password-login"
	if got != want {
		t.Errorf("slugifyHeading() = %q, want %q", got, want)
	}
}

// ---------- specExportIn: HTML format ----------

func TestSpecExportHTMLMultiShard(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth-system", exportShardAuth)
	setupCapabilitySpec(t, root, "billing", exportShardBilling)

	path, wrote, err := specExportIn(root, "", "html", "")
	if err != nil {
		t.Fatalf("specExportIn: %v", err)
	}
	if !wrote {
		t.Fatalf("specExportIn: wrote = false, want true")
	}
	if filepath.Base(path) != "specs.html" {
		t.Errorf("path = %q, want basename specs.html", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	out := string(data)

	for _, want := range []string{
		"Auth System",
		"Billing",
		`id="cap-auth-system"`,
		`id="cap-billing"`,
		"Handles authentication &amp; authorization for the &lt;platform&gt;.",
		"<code>429 Too Many Requests</code>",
		"| Attempt | Limit |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("export missing %q", want)
		}
	}
	if strings.Contains(out, "http://") || strings.Contains(out, "https://") {
		t.Errorf("export contains an external resource reference")
	}
}

func TestSpecExportHTMLSingleCapability(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth-system", exportShardAuth)
	setupCapabilitySpec(t, root, "billing", exportShardBilling)

	path, wrote, err := specExportIn(root, "billing", "html", "")
	if err != nil {
		t.Fatalf("specExportIn: %v", err)
	}
	if !wrote {
		t.Fatalf("specExportIn: wrote = false, want true")
	}
	if filepath.Base(path) != "billing.html" {
		t.Errorf("path = %q, want basename billing.html", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "Billing") {
		t.Errorf("export missing capability content")
	}
	if strings.Contains(out, "Auth System") || strings.Contains(out, `id="cap-auth-system"`) {
		t.Errorf("single-capability export leaked auth-system content")
	}
}

func TestSpecExportHTMLEscapingHostileInput(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "xss-check", exportShardXSS)

	path, wrote, err := specExportIn(root, "", "html", "")
	if err != nil || !wrote {
		t.Fatalf("specExportIn: wrote=%v err=%v", wrote, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("export did not escape hostile markup")
	}
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Errorf("export contains unescaped script tag")
	}
}

func TestSpecExportUnknownCapability(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth-system", exportShardAuth)

	_, wrote, err := specExportIn(root, "does-not-exist", "html", "")
	if err == nil {
		t.Fatalf("specExportIn: err = nil, want error for unknown capability")
	}
	if wrote {
		t.Errorf("specExportIn: wrote = true, want false on error")
	}
	if !strings.Contains(err.Error(), "available") || !strings.Contains(err.Error(), "auth-system") {
		t.Errorf("error %q does not list available capabilities", err.Error())
	}
}

func TestSpecExportEmptyState(t *testing.T) {
	root := t.TempDir()

	path, wrote, err := specExportIn(root, "", "html", "")
	if err != nil {
		t.Fatalf("specExportIn: %v", err)
	}
	if wrote {
		t.Errorf("specExportIn: wrote = true, want false for empty state")
	}
	if path != "" {
		t.Errorf("specExportIn: path = %q, want empty", path)
	}
	if _, statErr := os.Stat(filepath.Join(specsExportDirIn(root), "specs.html")); !os.IsNotExist(statErr) {
		t.Errorf("empty-state export wrote a file, want none")
	}
}

func TestSpecExportOutOverride(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "billing", exportShardBilling)

	override := filepath.Join(root, "custom", "myexport.html")
	path, wrote, err := specExportIn(root, "", "html", override)
	if err != nil || !wrote {
		t.Fatalf("specExportIn: wrote=%v err=%v", wrote, err)
	}
	wantAbs, _ := filepath.Abs(override)
	if path != wantAbs {
		t.Errorf("path = %q, want %q", path, wantAbs)
	}
	if _, statErr := os.Stat(override); statErr != nil {
		t.Errorf("override path not written: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(specsExportDirIn(root), "specs.html")); !os.IsNotExist(statErr) {
		t.Errorf("default location was also written, want only the --out override")
	}
}

func TestSpecExportInvalidFormat(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "billing", exportShardBilling)

	if _, _, err := specExportIn(root, "", "pdf", ""); err == nil {
		t.Fatalf("specExportIn: err = nil, want error for invalid format")
	}
}

// ---------- specExportIn: Markdown format ----------

func TestSpecExportMarkdownFormat(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "auth-system", exportShardAuth)
	setupCapabilitySpec(t, root, "billing", exportShardBilling)

	path, wrote, err := specExportIn(root, "", "md", "")
	if err != nil || !wrote {
		t.Fatalf("specExportIn: wrote=%v err=%v", wrote, err)
	}
	if filepath.Base(path) != "specs.md" {
		t.Errorf("path = %q, want basename specs.md", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	out := string(data)

	if strings.Contains(out, "git_hash:") || strings.Contains(out, "abc123") || strings.Contains(out, "def456") {
		t.Errorf("markdown export retained frontmatter, want stripped")
	}
	for _, want := range []string{
		"## Table of Contents",
		"[Auth System](#auth-system)",
		"[Password Login](#requirement-password-login)",
		"### Requirement: Password Login",
		"### Requirement: Invoice Generation",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown export missing %q", want)
		}
	}
}

// ---------- CRLF tolerance ----------

func TestSpecExportCRLFInput(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "billing", exportShardBillingCRLF)

	path, wrote, err := specExportIn(root, "", "html", "")
	if err != nil || !wrote {
		t.Fatalf("specExportIn: wrote=%v err=%v", wrote, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	out := string(data)
	if strings.Contains(out, "\r") {
		t.Errorf("export retained CRLF line endings")
	}
	if !strings.Contains(out, "<h4 class=\"scenario\">Scenario: cycle ends</h4>") {
		t.Errorf("export did not render CRLF-sourced scenario correctly")
	}
}

// ---------- unreadable shard surfaces a warning, not silent data loss ----------

func TestLoadSpecShardsInWarnsOnUnreadableShard(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "billing", exportShardBilling)

	// auth-system has a capability directory but no spec.md inside — simulates an
	// unreadable shard that must be skipped (so the export still succeeds) without
	// being silently dropped from a document meant to be a complete archive.
	if err := os.MkdirAll(filepath.Join(specsDirIn(root), "auth-system"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var shards []specExportShard
	var loadErr error
	stderr := captureStderr(func() {
		shards, loadErr = loadSpecShardsIn(root)
	})
	if loadErr != nil {
		t.Fatalf("loadSpecShardsIn: %v", loadErr)
	}

	if len(shards) != 1 || shards[0].Capability != "billing" {
		t.Fatalf("shards = %+v, want only billing", shards)
	}
	if !strings.Contains(stderr, "auth-system") {
		t.Errorf("stderr = %q, want it to name the skipped capability %q", stderr, "auth-system")
	}
	if !strings.Contains(stderr, "warning") {
		t.Errorf("stderr = %q, want a warning-labeled message", stderr)
	}
}

func TestSpecExportSucceedsDespiteUnreadableShard(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "billing", exportShardBilling)
	if err := os.MkdirAll(filepath.Join(specsDirIn(root), "auth-system"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var path string
	var wrote bool
	var err error
	stderr := captureStderr(func() {
		path, wrote, err = specExportIn(root, "", "html", "")
	})
	if err != nil || !wrote {
		t.Fatalf("specExportIn: wrote=%v err=%v", wrote, err)
	}
	if !strings.Contains(stderr, "auth-system") {
		t.Errorf("export command path did not surface the skipped-shard warning: stderr = %q", stderr)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read export: %v", readErr)
	}
	if !strings.Contains(string(data), "Billing") {
		t.Errorf("export missing the readable capability's content")
	}
}

// ---------- pending deltas excluded ----------

func TestSpecExportExcludesPendingDeltas(t *testing.T) {
	root := t.TempDir()
	setupCapabilitySpec(t, root, "billing", exportShardBilling)

	deltaDir := filepath.Join(root, ".claude", "feature", "feat-x", "spec-delta")
	if err := os.MkdirAll(deltaDir, 0o755); err != nil {
		t.Fatalf("mkdir delta dir: %v", err)
	}
	deltaContent := "## ADDED Requirements\n\n### Requirement: Should Not Appear\n\n" +
		"The system SHALL never appear in an export.\n\n#### Scenario: pending\n\n" +
		"- **WHEN** a delta is pending\n- **THEN** it SHALL NOT be exported\n"
	if err := os.WriteFile(filepath.Join(deltaDir, "billing.md"), []byte(deltaContent), 0o644); err != nil {
		t.Fatalf("write delta: %v", err)
	}

	path, wrote, err := specExportIn(root, "", "html", "")
	if err != nil || !wrote {
		t.Fatalf("specExportIn: wrote=%v err=%v", wrote, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if strings.Contains(string(data), "Should Not Appear") {
		t.Errorf("export contains pending delta content, want excluded")
	}
}
