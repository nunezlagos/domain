package main

import (
	"os"
	"path/filepath"
	"testing"
)

// DOMAINSERV-84: el instalador debe RECONCILIAR config stale de un install previo,
// no preservarla. Estos tests cubren los 2 casos que el self-check del doctor
// destapó en un install real.

func TestReconcileClaudeHook_MatcherStale_LoActualiza(t *testing.T) {
	hooks := map[string]any{
		"PostToolUse": []any{
			map[string]any{
				"matcher": "mcp__domain-mcp__domain_(orchestrate|flow_status)",
				"hooks":   []any{map[string]any{"type": "command", "command": "/h/post-orchestrate.sh"}},
			},
		},
	}
	exists, updated := reconcileClaudeHook(hooks, "PostToolUse", "/h/post-orchestrate.sh",
		"mcp__domain-mcp__domain_(orchestrate|flow_cancel)")
	if !exists || !updated {
		t.Fatalf("exists=%v updated=%v; want true,true", exists, updated)
	}
	entry := hooks["PostToolUse"].([]any)[0].(map[string]any)
	if got := entry["matcher"]; got != "mcp__domain-mcp__domain_(orchestrate|flow_cancel)" {
		t.Fatalf("matcher no reconciliado: %v", got)
	}
}

func TestReconcileClaudeHook_MatcherIgual_NoMuta(t *testing.T) {
	hooks := map[string]any{
		"PreToolUse": []any{
			map[string]any{
				"matcher": "Edit|Write|NotebookEdit|Bash",
				"hooks":   []any{map[string]any{"type": "command", "command": "/h/pre-edit.sh"}},
			},
		},
	}
	exists, updated := reconcileClaudeHook(hooks, "PreToolUse", "/h/pre-edit.sh", "Edit|Write|NotebookEdit|Bash")
	if !exists || updated {
		t.Fatalf("exists=%v updated=%v; want true,false", exists, updated)
	}
}

func TestReconcileClaudeHook_NoRegistrado_ReturnsFalse(t *testing.T) {
	exists, updated := reconcileClaudeHook(map[string]any{}, "PostToolUse", "/h/x.sh", "m")
	if exists || updated {
		t.Fatalf("exists=%v updated=%v; want false,false", exists, updated)
	}
}

func TestInstallOpencodePermission_ForzaDenySobreAsk(t *testing.T) {
	tmp := t.TempDir()
	mcp := filepath.Join(tmp, "opencode.json")
	// install previo dejó reset --hard como "ask" (no deny) + un catch-all del usuario
	if err := os.WriteFile(mcp, []byte(`{"permission":{"bash":{"git reset --hard *":"ask","*":"ask"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installOpencodePermission(Paths{OpencodeDir: tmp, OpencodeMCP: mcp}, "20260101000000"); err != nil {
		t.Fatal(err)
	}
	m, err := loadOrEmptyJSON(mcp)
	if err != nil {
		t.Fatal(err)
	}
	bash := m["permission"].(map[string]any)["bash"].(map[string]any)
	if got := bash["git reset --hard *"]; got != "deny" {
		t.Fatalf("reset --hard debe forzarse a deny, quedó: %v", got)
	}
	if got := bash["*"]; got != "ask" {
		t.Fatalf("el catch-all del usuario debe respetarse, quedó: %v", got)
	}
}
