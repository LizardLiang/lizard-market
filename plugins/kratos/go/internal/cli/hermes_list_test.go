package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupHermesChecklist creates a hermes-checklist.json at .claude/tmp/ inside tmpDir
// and returns the full path. Tests use os.Chdir(tmpDir) so findHermesChecklist picks it up.
func setupHermesChecklist(t *testing.T, tmpDir string, content map[string]interface{}) string {
	t.Helper()
	checklistDir := filepath.Join(tmpDir, ".claude", "tmp")
	if err := os.MkdirAll(checklistDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(checklistDir, "hermes-checklist.json")
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// chdirTo changes the working directory to dir and restores the original on cleanup.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

// runHermesList executes the hermes-list command with the given args and returns any error.
func runHermesList(args ...string) error {
	cmd := HermesListCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd.Execute()
}

// readTiers reads hermes-checklist.json and returns the tiers map.
func readTiers(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	tiers, _ := doc["tiers"].(map[string]interface{})
	return tiers
}

func TestHermesListCheck_ValidTier(t *testing.T) {
	tmpDir := t.TempDir()
	chdirTo(t, tmpDir)

	path := setupHermesChecklist(t, tmpDir, map[string]interface{}{
		"agent_id": "test-agent",
		"tiers":    allTiersFalse(),
	})

	if err := runHermesList("check", "T1_correct"); err != nil {
		t.Fatalf("check T1_correct: %v", err)
	}

	tiers := readTiers(t, path)

	if tiers["T1_correct"] != true {
		t.Error("T1_correct should be true after check")
	}
	if tiers["T2_safe"] != false {
		t.Error("T2_safe should still be false")
	}
	// All 8 keys must be present.
	for _, key := range tierOrder {
		if _, ok := tiers[key]; !ok {
			t.Errorf("tier key %q missing from output", key)
		}
	}
}

func TestHermesListCheck_AllTiers(t *testing.T) {
	tmpDir := t.TempDir()
	chdirTo(t, tmpDir)

	path := setupHermesChecklist(t, tmpDir, map[string]interface{}{
		"agent_id": "test-agent",
		"tiers":    allTiersFalse(),
	})

	for _, tier := range tierOrder {
		if err := runHermesList("check", tier); err != nil {
			t.Fatalf("check %s: %v", tier, err)
		}
	}

	tiers := readTiers(t, path)
	for _, key := range tierOrder {
		if tiers[key] != true {
			t.Errorf("tier %q should be true after check", key)
		}
	}
}

func TestHermesListCheck_InvalidTier(t *testing.T) {
	tmpDir := t.TempDir()
	chdirTo(t, tmpDir)

	path := setupHermesChecklist(t, tmpDir, map[string]interface{}{
		"agent_id": "test-agent",
		"tiers":    allTiersFalse(),
	})

	statBefore, _ := os.Stat(path)

	if err := runHermesList("check", "T9_bogus"); err == nil {
		t.Error("expected error for invalid tier, got nil")
	}

	// File must be unchanged.
	statAfter, _ := os.Stat(path)
	if statBefore.ModTime() != statAfter.ModTime() {
		t.Error("file was modified despite invalid tier")
	}
}

func TestHermesListCheck_MissingChecklist(t *testing.T) {
	tmpDir := t.TempDir()
	chdirTo(t, tmpDir)
	// No checklist file created.

	if err := runHermesList("check", "T1_correct"); err == nil {
		t.Error("expected error when checklist file is missing, got nil")
	}
}

func TestHermesListCheck_PreservesBlockCount(t *testing.T) {
	tmpDir := t.TempDir()
	chdirTo(t, tmpDir)

	path := setupHermesChecklist(t, tmpDir, map[string]interface{}{
		"agent_id":    "test-agent",
		"block_count": 2,
		"tiers":       allTiersFalse(),
	})

	if err := runHermesList("check", "T1_correct"); err != nil {
		t.Fatalf("check T1_correct: %v", err)
	}

	data, _ := os.ReadFile(path)
	var doc map[string]interface{}
	json.Unmarshal(data, &doc)

	if bc, _ := doc["block_count"].(float64); int(bc) != 2 {
		t.Errorf("block_count should be preserved as 2, got %v", doc["block_count"])
	}
}

func TestHermesListCheck_ShortForm(t *testing.T) {
	tmpDir := t.TempDir()
	chdirTo(t, tmpDir)

	path := setupHermesChecklist(t, tmpDir, map[string]interface{}{
		"agent_id": "test-agent",
		"tiers":    allTiersFalse(),
	})

	// T1 should resolve to T1_correct.
	if err := runHermesList("check", "T1"); err != nil {
		t.Fatalf("check T1: %v", err)
	}

	tiers := readTiers(t, path)
	if tiers["T1_correct"] != true {
		t.Error("T1_correct should be true after `check T1`")
	}
	if tiers["T2_safe"] != false {
		t.Error("T2_safe should still be false")
	}
}

func TestHermesListCheck_NoTmpFileAfterWrite(t *testing.T) {
	tmpDir := t.TempDir()
	chdirTo(t, tmpDir)

	setupHermesChecklist(t, tmpDir, map[string]interface{}{
		"agent_id": "test-agent",
		"tiers":    allTiersFalse(),
	})

	if err := runHermesList("check", "T1_correct"); err != nil {
		t.Fatalf("check T1_correct: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join(tmpDir, ".claude", "tmp", "*.tmp"))
	if len(matches) > 0 {
		t.Errorf("unexpected .tmp files remain: %v", matches)
	}
}
