//go:build !race

// TestSkill_TypeCleanup_MigrationExists verifica que el archivo de
// migration para Día 1 del RFC 0008 (Opción A: skill_type_cleanup)
// existe y tiene la lógica correcta.
//
// Día 1 = migration que convierte stubs a 'prompt'. Día 7 = code
// change que rechaza los 3 tipos en Create (deferido, se haría en
// commit separado para no romper tests existentes que crean skills
// con TypeCode/TypeAPI/TypeMCPTool en runners).
package admin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSkill_TypeCleanup_MigrationExists(t *testing.T) {

	cwd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot := cwd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
			break
		}
		repoRoot = filepath.Dir(repoRoot)
	}

	migPath := filepath.Join(repoRoot,
		"internal", "migrate", "migrations",
		"000144_skill_type_cleanup.up.sql")
	body, err := os.ReadFile(migPath)
	require.NoError(t, err, "migration debe existir en %s", migPath)
	text := string(body)


	require.Contains(t, text, "UPDATE skills",
		"migration debe contener UPDATE skills")
	require.Contains(t, text, "'api'",
		"migration debe incluir 'api' en la lista de tipos deprecated")
	require.Contains(t, text, "'code'",
		"migration debe incluir 'code' en la lista de tipos deprecated")
	require.Contains(t, text, "'mcp_tool'",
		"migration debe incluir 'mcp_tool' en la lista de tipos deprecated")
	require.Contains(t, text, "'prompt'",
		"migration debe convertir al tipo canónico 'prompt'")
	require.Contains(t, text, "deleted_at IS NULL",
		"migration debe excluir soft-deleted")
}

// TestSkill_TypeCleanup_MigrationReversible verifica que el .down.sql
// existe y menciona el rollback path (al menos como best-effort).
func TestSkill_TypeCleanup_MigrationReversible(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot := cwd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
			break
		}
		repoRoot = filepath.Dir(repoRoot)
	}

	downPath := filepath.Join(repoRoot,
		"internal", "migrate", "migrations",
		"000144_skill_type_cleanup.down.sql")
	body, err := os.ReadFile(downPath)
	require.NoError(t, err, "down migration debe existir en %s", downPath)
	text := strings.ToLower(string(body))

	require.Contains(t, text, "skill_type_backup",
		"down debe referenciar la tabla backup del up")
}
