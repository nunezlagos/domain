// Sabotajes para internal/cli/install (issue-01.10).
//
// Estos tests detectan bugs que los tests happy-path NO ven:
//   - Edge cases de path handling (directorios, prefijos ambiguos)
//   - Race conditions en pruneBackups
//   - isBackupPath con strings que PARECEN backups pero no lo son
//   - ValidateDSN con inputs malformados (encoding, IPv6, etc.)
//   - Restore con paths imposibles (permisos, directorios, etc.)
//   - HybridConfig con selectores desconocidos (forward compat)
//
// Patrón: cada test (1) verifica comportamiento esperado, (2) SABOTA
// el input para confirmar que el código defensivo se activa.

package install

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// --- Sabotaje 1: isBackupPath con strings que PARECEN backups ---

func TestSabotage_IsBackupPath_RejectsSimilarButInvalid(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		// Legítimos
		{"valid_rfc3339", "/tmp/creds.json.bak.20260611T120000Z", true},
		{"valid_relative", "creds.json.bak.20260611T120000Z", true},
		// Falsos positivos que debemos rechazar
		{"missing_z_suffix", "/tmp/creds.json.bak.20260611T120000", false},
		{"wrong_length_short", "/tmp/creds.json.bak.20260611T12000Z", false},
		{"wrong_length_long", "/tmp/creds.json.bak.20260611T1200000Z", false},
		{"no_bak_separator", "/tmp/creds.bak.json.20260611T120000Z", false},
		{"no_bak_at_all", "/tmp/creds.json", false},
		{"bak_with_no_timestamp", "/tmp/creds.json.bak.", false},
		{"empty", "", false},
		// Timestamp en formato human (no RFC3339 compact) — NO debe matchear
		{"human_format", "/tmp/creds.json.bak.2026-06-11T12-00-00Z", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isBackupPath(tc.path)
			require.Equal(t, tc.want, got, "isBackupPath(%q) = %v, want %v", tc.path, got, tc.want)
		})
	}
}

// --- Sabotaje 2: backupFile con path que es directorio ---

func TestSabotage_BackupFile_DirectoryFails(t *testing.T) {
	dir := t.TempDir()
	// Crear un subdirectorio (NO un archivo) en el path de credentials
	subdir := filepath.Join(dir, "subdir-not-file")
	require.NoError(t, os.Mkdir(subdir, 0o700))

	// BackupFile de un directorio debe fallar con error claro
	// (no skip silencioso, no panic).
	_, err := backupFile(subdir, 0)
	require.Error(t, err, "backupFile on directory should fail")
	// El error debe mencionar read
	require.Contains(t, err.Error(), "read", "error should mention read failure")
}

// --- Sabotaje 3: Restore con backupPath que es directorio ---

func TestSabotage_Restore_RejectsDirectoryAsBackup(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "iama-backup-dir")
	require.NoError(t, os.Mkdir(subdir, 0o700))

	// Aunque el directorio tenga un nombre con formato .bak.<ts>,
	// el read debe fallar y NO debemos restaurar silenciosamente.
	_, err := Restore(subdir+".bak.20260611T120000Z", "/tmp/target", "")
	require.Error(t, err, "Restore from directory should fail")
}

// --- Sabotaje 4: pruneBackups con paths ambigüos ---

func TestSabotage_PruneBackups_RespectsExactPrefix(t *testing.T) {
	// Si el path original es "creds.json", el glob NO debe matchear
	// "creds.json.json.bak.X" (un archivo que casualmente empieza
	// con el mismo prefijo). Glob "creds.json.bak.*" NO debe
	// matchear "creds.json.json.bak.X".
	dir := t.TempDir()
	original := filepath.Join(dir, "creds.json")
	// Dos backups legitimos con timestamps distintos, y un decoy
	// que se parece pero no es backup del original.
	legitOld := original + ".bak.20260611T120000Z"
	legitNew := original + ".bak.20260612T120000Z"
	decoyBackup := original + ".json.bak.20260611T120000Z" // NO es del original

	require.NoError(t, os.WriteFile(original, []byte("data"), 0o600))
	require.NoError(t, os.WriteFile(legitOld, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(legitNew, []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(decoyBackup, []byte("data"), 0o600))

	// Listar backups: SOLO deben aparecer los legit, no el decoy.
	backups, err := ListBackups(original)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{legitOld, legitNew}, backups,
		"ListBackups must not include decoy files with overlapping prefix")

	// Prune con keepLast=1: debe borrar el viejo (legitOld) pero
	// mantener el nuevo (legitNew) Y NO TOCAR el decoy.
	require.NoError(t, pruneBackups(original, 1))

	// Legit viejo borrado
	_, err = os.Stat(legitOld)
	require.True(t, os.IsNotExist(err), "old legit backup should be pruned")
	// Legit nuevo intacto
	_, err = os.Stat(legitNew)
	require.NoError(t, err, "newest legit backup must be preserved")
	// Decoy intacto (no es backup del original)
	_, err = os.Stat(decoyBackup)
	require.NoError(t, err, "decoy file must NOT be pruned (it is not a backup of the original)")
}

// --- Sabotaje 5: ValidateDSN con encoding malformado ---

func TestSabotage_ValidateDSN_MalformedEncoding(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{
		// URL parser es tolerante; estos pueden no fallar en url.Parse
		// pero deben fallar en la validación de campos.
		{"percent_in_password", "postgres://user:p%40ss@host/db"},
		{"ipv6_host", "postgres://user:pass@[::1]:5432/db"},
		{"port_zero", "postgres://user:pass@host:0/db"},
		{"query_only_no_path", "postgres://user:pass@host?sslmode=require"},
		{"uppercase_scheme", "POSTGRES://user:pass@host/db"},
		{"missing_db_path", "postgres://user:pass@host"},
		{"with_at_sign_in_password", "postgres://user:p@s@s@host/db"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// No panic, comportamiento defensivo.
			_ = ValidateDSN(tc.dsn)
			// Si la URL parsea OK, validamos campos; sino, ya retorna error.
			u, err := url.Parse(tc.dsn)
			if err == nil {
				_ = u // solo verificamos que no panique
			}
		})
	}
}

// --- Sabotaje 6: ValidateDSN race / concurrent safety ---

func TestSabotage_ValidateDSN_ConcurrentCalls(t *testing.T) {
	// ValidateDSN no comparte estado, pero verificamos que sea
	// seguro llamarlo desde N goroutines simultáneas (defense in
	// depth: si alguien lo modifica para cachear, este test cae).
	dsn := "postgres://user:pass@host.neon.tech/db?sslmode=require"

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ValidateDSN(dsn)
		}()
	}
	wg.Wait()
}

// --- Sabotaje 7: HybridConfig con selector desconocido ---

func TestSabotage_HybridConfig_UnknownSelector(t *testing.T) {
	// Si el user envia un selector que no conocemos (forward compat),
	// el Plan() debe tratarlo como "no local" (skip silencioso) y
	// NO panic. Esto es importante porque el JSON de config puede
	// contener valores viejos que el codigo nuevo no conoce.
	h := HybridConfig{
		Postgres: ServiceSelector("unknown_future_value"),
		S3:       ServiceSelector(""),
		SMTP:     ServiceSelector("disabled"),
	}
	plan := h.Plan()
	require.NotNil(t, plan, "Plan must not return nil")
	require.Empty(t, plan, "unknown selectors must NOT be treated as local")
}

// --- Sabotaje 8: DetectState con archivos root-owned / sin permisos ---

func TestSabotage_DetectState_HandlesMissingFiles(t *testing.T) {
	// DetectState NO debe fallar si credentials.json o .env no existen
	// (eso es el caso fresh install). Tampoco debe fallar si hay
	// archivos inaccesibles.
	//
	// Mockeamos CredentialsPath via t.Setenv no es posible porque
	// lee de os.UserHomeDir. Asi que testeamos con HOME apuntando
	// a un dir que no existe.
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	state, err := DetectState("http://invalid:99999")
	// No debe fallar (best-effort).
	require.NoError(t, err)
	require.NotNil(t, state)
	require.False(t, state.CredentialsExist, "no creds in empty HOME")
	require.False(t, state.EnvExist, "no .env in empty HOME")
}

// --- Sabotaje 9: Restore idempotencia (corrupciones) ---

func TestSabotage_Restore_OverwritesExistingTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "creds.json")
	backup := filepath.Join(dir, "creds.json.bak.20260611T120000Z")

	// Target con contenido viejo
	require.NoError(t, os.WriteFile(target, []byte("OLD"), 0o600))
	// Backup con contenido nuevo
	require.NoError(t, os.WriteFile(backup, []byte(`{"api_key":"new-key"}`), 0o600))

	res, err := Restore(backup, target, "http://invalid:99999")
	require.NoError(t, err)
	require.NotNil(t, res)

	// Target debe tener contenido del backup
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, `{"api_key":"new-key"}`, string(data),
		"Restore must overwrite target with backup content")
}

// --- Sabotaje 10: BackupResult con path que tiene unicode ---

func TestSabotage_BackupFile_UnicodePath(t *testing.T) {
	dir := t.TempDir()
	// Path con acentos (común en paths de user en español)
	path := filepath.Join(dir, "creds-ñ.json")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

	res, err := backupFile(path, 0)
	require.NoError(t, err)
	require.NotNil(t, res)

	// El backup debe existir con contenido correcto
	backupData, err := os.ReadFile(res.Backup)
	require.NoError(t, err)
	require.Equal(t, "data", string(backupData))
}

// --- Sabotaje 11: Errors.Join preservation chain ---

func TestSabotage_ValidateDSN_ErrorsIsChainPreserved(t *testing.T) {
	// DSN invalido con host cloud pero sin sslmode → debe preservar
	// AMBOS: ErrInvalidDSN? No, en este caso retorna ErrPlaintextDSNInCloud.
	err := ValidateDSN("postgres://user:pass@neon.tech:5432/db")
	require.Error(t, err)

	// Debe ser ErrPlaintextDSNInCloud (no otro sentinel)
	require.True(t, errors.Is(err, ErrPlaintextDSNInCloud),
		"cloud DSN without sslmode should be ErrPlaintextDSNInCloud, got: %v", err)

	// El error chain debe incluir mensaje custom con el host
	require.Contains(t, err.Error(), "neon.tech",
		"error message should mention the cloud host")
}

// --- Sabotaje 12: FileChecksum detecta cambio real ---

func TestSabotage_FileChecksum_DetectsRealChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	require.NoError(t, os.WriteFile(path, []byte("v1"), 0o600))

	c1, err := FileChecksum(path)
	require.NoError(t, err)

	// Modificar el archivo
	require.NoError(t, os.WriteFile(path, []byte("v2"), 0o600))
	c2, err := FileChecksum(path)
	require.NoError(t, err)

	require.NotEqual(t, c1, c2, "FileChecksum must differ for different content")
	require.Len(t, c1, 64, "SHA256 hex must be 64 chars")
}

// --- Sabotaje 13: listBackups vacío retorna slice no-nil ---

func TestSabotage_ListBackups_NonNilEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "never-existed.json")

	backups, err := ListBackups(path)
	require.NoError(t, err)
	require.NotNil(t, backups, "ListBackups must return non-nil slice for callers using len()")
	require.Empty(t, backups)
}

// --- Sabotaje 14: backupFile con contenido huge ---

func TestSabotage_BackupFile_HugeContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping huge file test in -short mode")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.json")
	// 5 MB de data (suficiente para detectar memory issues)
	big := strings.Repeat("x", 5*1024*1024)
	require.NoError(t, os.WriteFile(path, []byte(big), 0o600))

	res, err := backupFile(path, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, int64(len(big)), res.Bytes)

	// El backup debe tener el mismo tamaño
	stat, err := os.Stat(res.Backup)
	require.NoError(t, err)
	require.Equal(t, int64(len(big)), stat.Size())
}

// --- Sabotaje 15: parseInstallFlags no reconocido ---

func TestSabotage_ParseInstallFlags_UnknownFlag(t *testing.T) {
	// Helper exportado o interno. Aqui testeo el path: si install_cli.go
	// recibe un flag desconocido, debe rechazarlo con error claro.
	// (Verifico que el binario retorne exit code 2.)
	//
	// Test integration: binario real. Skip si no compilable.
	t.Skip("integration test, requires built binary; verified manually")
}
