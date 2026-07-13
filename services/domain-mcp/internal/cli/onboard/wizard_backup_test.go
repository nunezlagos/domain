package onboard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSaveCredentialsDefault_ExistingFile_CreatesBakWithTimestamp verifica
// que al sobrescribir un credentials.json existente el backup usa la
// convención .bak.<ts> (reconocible por domain restore), no un .bak fijo.
func TestSaveCredentialsDefault_ExistingFile_CreatesBakWithTimestamp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := CredentialsPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`{"api_key":"old"}`), 0o600))

	require.NoError(t, SaveCredentialsDefault(&Credentials{APIKey: "new"}))

	baks, err := filepath.Glob(path + ".bak.*")
	require.NoError(t, err)
	require.Len(t, baks, 1, "debe existir un backup .bak.<ts>")

	fixed, err := os.Stat(path + ".bak")
	require.Truef(t, os.IsNotExist(err), "no debe usar el .bak de nombre fijo (stat: %v)", fixed)
}
