package workflowimport_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/workflowimport"
)

func setupFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Estructura fixture: archivos de Claude Code, OpenCode, Cursor.
	files := map[string]string{
		"CLAUDE.md":             "# Claude global\n\nTono profesional.",
		".claude/rules/git.md":  "# Git rules\n\nConventional commits.",
		".claude/rules/db.md":   "# DB rules\n\nUUID + TIMESTAMPTZ.",
		".opencode/agents.md":   "# OpenCode agents\n\nDef agents.",
		".cursor/rules.md":      "# Cursor rules",
		".cursorrules":          "Use TDD.",
		".windsurfrules":        "Be concise.",
		"AGENTS.md":             "# Agents generic",
		"src/main.go":           "package main", // NO debe detectarse
		"node_modules/foo.md":   "skip",         // NO debe entrar
		"README.md":             "# Project",    // NO matchea patterns
	}
	for path, content := range files {
		abs := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
	}
	return root
}

func TestScanner_DetectsAllKnownPatterns(t *testing.T) {
	root := setupFixture(t)
	scanner := &workflowimport.Scanner{ProjectRoot: root}

	files, err := scanner.Detect(true)
	require.NoError(t, err)

	gotPaths := make(map[string]string)
	for _, f := range files {
		gotPaths[f.RelPath] = f.SourceTool
	}

	// Esperados (8 archivos):
	require.Equal(t, "claude-code", gotPaths["CLAUDE.md"])
	require.Equal(t, "claude-code", gotPaths[".claude/rules/git.md"])
	require.Equal(t, "claude-code", gotPaths[".claude/rules/db.md"])
	require.Equal(t, "opencode", gotPaths[".opencode/agents.md"])
	require.Equal(t, "cursor", gotPaths[".cursor/rules.md"])
	require.Equal(t, "cursor", gotPaths[".cursorrules"])
	require.Equal(t, "windsurf", gotPaths[".windsurfrules"])
	require.Equal(t, "generic", gotPaths["AGENTS.md"])

	// NO esperados:
	_, hasMain := gotPaths["src/main.go"]
	require.False(t, hasMain, "no debe detectar .go files")
	_, hasReadme := gotPaths["README.md"]
	require.False(t, hasReadme, "README.md no es un .md de IA")
	for path := range gotPaths {
		require.NotContains(t, path, "node_modules", "node_modules debe estar excluido")
	}
}

func TestScanner_SnapshotContent_PopulatesContentAndHash(t *testing.T) {
	root := setupFixture(t)
	scanner := &workflowimport.Scanner{ProjectRoot: root}

	files, err := scanner.Detect(true)
	require.NoError(t, err)

	for _, f := range files {
		require.NotEmpty(t, f.ContentHash, "hash debe estar populado")
		require.NotEmpty(t, f.Content, "content debe estar populado con snapshot=true")
		require.Equal(t, int64(len(f.Content)), f.SizeBytes)
	}
}

func TestScanner_SnapshotFalse_OnlyMetadata(t *testing.T) {
	root := setupFixture(t)
	scanner := &workflowimport.Scanner{ProjectRoot: root}

	files, err := scanner.Detect(false)
	require.NoError(t, err)

	for _, f := range files {
		require.NotEmpty(t, f.ContentHash)
		require.Empty(t, f.Content, "content debe estar vacío con snapshot=false")
	}
}

func TestScanner_RootNotFound(t *testing.T) {
	scanner := &workflowimport.Scanner{ProjectRoot: "/nonexistent/path"}
	_, err := scanner.Detect(true)
	require.ErrorIs(t, err, workflowimport.ErrRootNotFound)
}

// Sabotaje: archivo .md fuera de los patterns conocidos NO debe detectarse
// aunque sea similar (ej. claude_thoughts.md, .claudia.md).
func TestSabotage_NonStandardMDPathsIgnored(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "claude_thoughts.md"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".claudia.md"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "CLAUDIA.md"), []byte("x"), 0o644))

	scanner := &workflowimport.Scanner{ProjectRoot: root}
	files, err := scanner.Detect(true)
	require.NoError(t, err)
	require.Empty(t, files, "patterns no estándar NO deben matchear")
}
