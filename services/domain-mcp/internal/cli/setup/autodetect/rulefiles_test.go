package autodetect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRuleFiles_FindsPresentOnes(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "CLAUDE.md"), "reglas")
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "agentes")
	mustWrite(t, filepath.Join(dir, ".cursorrules"), "x")
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := DetectRuleFiles(dir)
	want := map[string]bool{"CLAUDE.md": true, "AGENTS.md": true, ".cursorrules": true, ".claude": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %d entries", got, len(want))
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("inesperado en resultado: %q", g)
		}
	}
}

func TestDetectRuleFiles_EmptyWhenNone(t *testing.T) {
	dir := t.TempDir()
	if got := DetectRuleFiles(dir); len(got) != 0 {
		t.Fatalf("esperaba vacío en dir sin reglas, got %v", got)
	}
}

func TestDetectRuleFiles_NestedCopilot(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, ".github", "copilot-instructions.md"), "gh")
	got := DetectRuleFiles(dir)
	found := false
	for _, g := range got {
		if g == ".github/copilot-instructions.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no detectó .github/copilot-instructions.md, got %v", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
