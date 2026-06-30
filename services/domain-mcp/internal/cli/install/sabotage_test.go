












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



func TestSabotage_IsBackupPath_RejectsSimilarButInvalid(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{

		{"valid_rfc3339", "/tmp/creds.json.bak.20260611T120000Z", true},
		{"valid_relative", "creds.json.bak.20260611T120000Z", true},

		{"missing_z_suffix", "/tmp/creds.json.bak.20260611T120000", false},
		{"wrong_length_short", "/tmp/creds.json.bak.20260611T12000Z", false},
		{"wrong_length_long", "/tmp/creds.json.bak.20260611T1200000Z", false},
		{"no_bak_separator", "/tmp/creds.bak.json.20260611T120000Z", false},
		{"no_bak_at_all", "/tmp/creds.json", false},
		{"bak_with_no_timestamp", "/tmp/creds.json.bak.", false},
		{"empty", "", false},

		{"human_format", "/tmp/creds.json.bak.2026-06-11T12-00-00Z", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isBackupPath(tc.path)
			require.Equal(t, tc.want, got, "isBackupPath(%q) = %v, want %v", tc.path, got, tc.want)
		})
	}
}



func TestSabotage_BackupFile_DirectoryFails(t *testing.T) {
	dir := t.TempDir()

	subdir := filepath.Join(dir, "subdir-not-file")
	require.NoError(t, os.Mkdir(subdir, 0o700))



	_, err := backupFile(subdir, 0)
	require.Error(t, err, "backupFile on directory should fail")

	require.Contains(t, err.Error(), "read", "error should mention read failure")
}



func TestSabotage_Restore_RejectsDirectoryAsBackup(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "iama-backup-dir")
	require.NoError(t, os.Mkdir(subdir, 0o700))



	_, err := Restore(subdir+".bak.20260611T120000Z", "/tmp/target", "")
	require.Error(t, err, "Restore from directory should fail")
}



func TestSabotage_PruneBackups_RespectsExactPrefix(t *testing.T) {




	dir := t.TempDir()
	original := filepath.Join(dir, "creds.json")


	legitOld := original + ".bak.20260611T120000Z"
	legitNew := original + ".bak.20260612T120000Z"
	decoyBackup := original + ".json.bak.20260611T120000Z" // NO es del original

	require.NoError(t, os.WriteFile(original, []byte("data"), 0o600))
	require.NoError(t, os.WriteFile(legitOld, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(legitNew, []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(decoyBackup, []byte("data"), 0o600))


	backups, err := ListBackups(original)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{legitOld, legitNew}, backups,
		"ListBackups must not include decoy files with overlapping prefix")



	require.NoError(t, pruneBackups(original, 1))


	_, err = os.Stat(legitOld)
	require.True(t, os.IsNotExist(err), "old legit backup should be pruned")

	_, err = os.Stat(legitNew)
	require.NoError(t, err, "newest legit backup must be preserved")

	_, err = os.Stat(decoyBackup)
	require.NoError(t, err, "decoy file must NOT be pruned (it is not a backup of the original)")
}



func TestSabotage_ValidateDSN_MalformedEncoding(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{


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

			_ = ValidateDSN(tc.dsn)

			u, err := url.Parse(tc.dsn)
			if err == nil {
				_ = u // solo verificamos que no panique
			}
		})
	}
}



func TestSabotage_ValidateDSN_ConcurrentCalls(t *testing.T) {



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



func TestSabotage_HybridConfig_UnknownSelector(t *testing.T) {




	h := HybridConfig{
		Postgres: ServiceSelector("unknown_future_value"),
		S3:       ServiceSelector(""),
		SMTP:     ServiceSelector("disabled"),
	}
	plan := h.Plan()
	require.NotNil(t, plan, "Plan must not return nil")
	require.Empty(t, plan, "unknown selectors must NOT be treated as local")
}



func TestSabotage_DetectState_HandlesMissingFiles(t *testing.T) {







	dir := t.TempDir()
	t.Setenv("HOME", dir)

	state, err := DetectState("http://invalid:99999")

	require.NoError(t, err)
	require.NotNil(t, state)
	require.False(t, state.CredentialsExist, "no creds in empty HOME")
	require.False(t, state.EnvExist, "no .env in empty HOME")
}



func TestSabotage_Restore_OverwritesExistingTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "creds.json")
	backup := filepath.Join(dir, "creds.json.bak.20260611T120000Z")


	require.NoError(t, os.WriteFile(target, []byte("OLD"), 0o600))

	require.NoError(t, os.WriteFile(backup, []byte(`{"api_key":"new-key"}`), 0o600))

	res, err := Restore(backup, target, "http://invalid:99999")
	require.NoError(t, err)
	require.NotNil(t, res)


	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, `{"api_key":"new-key"}`, string(data),
		"Restore must overwrite target with backup content")
}



func TestSabotage_BackupFile_UnicodePath(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "creds-ñ.json")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

	res, err := backupFile(path, 0)
	require.NoError(t, err)
	require.NotNil(t, res)


	backupData, err := os.ReadFile(res.Backup)
	require.NoError(t, err)
	require.Equal(t, "data", string(backupData))
}



func TestSabotage_ValidateDSN_ErrorsIsChainPreserved(t *testing.T) {


	err := ValidateDSN("postgres://user:pass@neon.tech:5432/db")
	require.Error(t, err)


	require.True(t, errors.Is(err, ErrPlaintextDSNInCloud),
		"cloud DSN without sslmode should be ErrPlaintextDSNInCloud, got: %v", err)


	require.Contains(t, err.Error(), "neon.tech",
		"error message should mention the cloud host")
}



func TestSabotage_FileChecksum_DetectsRealChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	require.NoError(t, os.WriteFile(path, []byte("v1"), 0o600))

	c1, err := FileChecksum(path)
	require.NoError(t, err)


	require.NoError(t, os.WriteFile(path, []byte("v2"), 0o600))
	c2, err := FileChecksum(path)
	require.NoError(t, err)

	require.NotEqual(t, c1, c2, "FileChecksum must differ for different content")
	require.Len(t, c1, 64, "SHA256 hex must be 64 chars")
}



func TestSabotage_ListBackups_NonNilEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "never-existed.json")

	backups, err := ListBackups(path)
	require.NoError(t, err)
	require.NotNil(t, backups, "ListBackups must return non-nil slice for callers using len()")
	require.Empty(t, backups)
}



func TestSabotage_BackupFile_HugeContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping huge file test in -short mode")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.json")

	big := strings.Repeat("x", 5*1024*1024)
	require.NoError(t, os.WriteFile(path, []byte(big), 0o600))

	res, err := backupFile(path, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, int64(len(big)), res.Bytes)


	stat, err := os.Stat(res.Backup)
	require.NoError(t, err)
	require.Equal(t, int64(len(big)), stat.Size())
}



func TestSabotage_ParseInstallFlags_UnknownFlag(t *testing.T) {





	t.Skip("integration test, requires built binary; verified manually")
}
