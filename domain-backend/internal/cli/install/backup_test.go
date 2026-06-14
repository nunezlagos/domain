package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// === Backup ===

func TestBackup_CreatesBakWithTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))

	res, err := backupFile(path, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, path, res.Path)
	require.True(t, strings.HasPrefix(res.Backup, path+".bak."),
		"backup debe empezar con %s.bak., got %s", path, res.Backup)
	require.Equal(t, int64(len("original")), res.Bytes)

	// El backup existe y tiene el contenido original
	data, err := os.ReadFile(res.Backup)
	require.NoError(t, err)
	require.Equal(t, "original", string(data))
}

func TestBackup_SkipsIfFileNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope.txt")
	res, err := backupFile(path, 0)
	require.NoError(t, err)
	require.Nil(t, res, "archivo inexistente debe retornar nil sin error")
}

func TestBackup_PrunesOldBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	// Crear 5 backups manuales con timestamps distintos
	for i := 0; i < 5; i++ {
		ts := time.Now().UTC().Add(time.Duration(i) * time.Second).Format("20060102T150405Z")
		backup := path + ".bak." + ts
		require.NoError(t, os.WriteFile(backup, []byte("v"+string(rune('0'+i))), 0o600))
	}
	// keepLast=2: debe borrar los 3 mas viejos, dejar los 2 mas nuevos
	_, err := backupFile(path, 2) // 6to backup
	// backupFile lee el archivo (que no existe en este test), retorna nil
	// Asi que el prune no se llama. Test invalido, ajusto:
	require.NoError(t, err)

	// Test mas realista: creo el archivo, luego backup, luego verifico
	// que keepLast funciona
	require.NoError(t, os.WriteFile(path, []byte("current"), 0o600))
	// Los 5 backups manuales + 1 nuevo = 6 totales
	_, err = backupFile(path, 2)
	require.NoError(t, err)

	// Despues de backupFile con keepLast=2, deben quedar 2 backups
	matches, _ := filepath.Glob(path + ".bak.*")
	require.Len(t, matches, 2, "deben quedar 2 backups, hay %d", len(matches))
}

func TestIsBackupPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/home/user/.config/domain/credentials.json.bak.20260611T120000Z", true},
		{".env.bak.20260101T000000Z", true},
		{"/some/path.bak.99999999T999999Z", true},
		{"/some/path.txt", false},
		{"/some/path.txt.bak", false},         // sin timestamp
		{"/some/path.txt.bak.2026", false},    // timestamp incompleto
		{"/some/path.txt.bak.20260101T120000", false}, // sin Z final
		{"/some/path.txt.bak.foo", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			require.Equal(t, tc.want, isBackupPath(tc.path))
		})
	}
}

// === IsDomainManaged ===

func TestIsDomainManaged_WithMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	content := "# Project rules\n\n" + DomainManagedMarker + "\n\nUse domain MCP."
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	ok, err := IsDomainManaged(path)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestIsDomainManaged_WithoutMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	require.NoError(t, os.WriteFile(path, []byte("# User's own rules\nno domain here"), 0o600))

	ok, err := IsDomainManaged(path)
	require.NoError(t, err)
	require.False(t, ok, "archivo sin marker debe retornar false (no es managed por domain)")
}

func TestIsDomainManaged_FileNotExist(t *testing.T) {
	ok, err := IsDomainManaged("/nonexistent/path")
	require.Error(t, err)
	require.False(t, ok)
}

// === ParseCredentials ===

func TestParseCredentials_Valid(t *testing.T) {
	creds := ParsedCredentials{APIKey: "domk_live_TEST123"}
	data, _ := json.Marshal(creds)
	parsed, err := ParseCredentials(data)
	require.NoError(t, err)
	require.Equal(t, "domk_live_TEST123", parsed.APIKey)
}

func TestParseCredentials_InvalidJSON(t *testing.T) {
	_, err := ParseCredentials([]byte("not json"))
	require.Error(t, err)
}

// === Restore ===

func TestRestore_WritesFromBackup(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "creds.json")
	backup := original + ".bak.20260611T120000Z"
	require.NoError(t, os.WriteFile(original, []byte("current"), 0o600))
	require.NoError(t, os.WriteFile(backup, []byte("backup content"), 0o600))

	res, err := Restore(backup, original, "")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, backup, res.Backup)
	require.Equal(t, original, res.Target)
	require.Equal(t, int64(len("backup content")), res.Bytes)

	// Target debe tener el contenido del backup
	data, _ := os.ReadFile(original)
	require.Equal(t, "backup content", string(data))
}

func TestRestore_RejectsNonBackupPath(t *testing.T) {
	_, err := Restore("/some/random/file.txt", "/some/target", "")
	require.ErrorIs(t, err, ErrNoBackup)
}

func TestRestore_BackupNotExist(t *testing.T) {
	_, err := Restore("/nonexistent/file.bak.20260611T120000Z", "/target", "")
	require.Error(t, err)
}

// === FileChecksum ===

func TestFileChecksum_Deterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o600))
	a, _ := FileChecksum(path)
	b, _ := FileChecksum(path)
	require.Equal(t, a, b)
	require.Len(t, a, 64, "sha256 hex = 64 chars")
}

func TestFileChecksum_DifferentContent_DifferentHash(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(a, []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("world"), 0o600))
	ha, _ := FileChecksum(a)
	hb, _ := FileChecksum(b)
	require.NotEqual(t, ha, hb)
}

// === ListBackups ===

func TestListBackups_ReturnsAllBackups(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "f.txt")
	for i := 0; i < 3; i++ {
		ts := time.Now().UTC().Add(time.Duration(i) * time.Second).Format("20060102T150405Z")
		require.NoError(t, os.WriteFile(original+".bak."+ts, []byte("v"), 0o600))
	}
	got, err := ListBackups(original)
	require.NoError(t, err)
	require.Len(t, got, 3)
}

func TestListBackups_NoBackups_EmptySlice(t *testing.T) {
	dir := t.TempDir()
	got, err := ListBackups(filepath.Join(dir, "nope.txt"))
	require.NoError(t, err)
	require.Empty(t, got)
}
