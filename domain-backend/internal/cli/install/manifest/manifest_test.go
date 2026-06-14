package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadManifest_NotExists(t *testing.T) {
	m, err := ReadManifest("/nonexistent/path.json")
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, 1, m.Version)
	require.Empty(t, m.Installs)
}

func TestReadManifest_Valid(t *testing.T) {
	content := `{"version":1,"installs":[{"install_id":"abc","entries":[{"id":"e1","type":"file_create","path":"/tmp/x"}]}]}`
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	m, err := ReadManifest(path)
	require.NoError(t, err)
	require.Len(t, m.Installs, 1)
	require.Len(t, m.Installs[0].Entries, 1)
	require.Equal(t, "e1", m.Installs[0].Entries[0].ID)
}

func TestReadManifest_Corrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(`{not json}`), 0o644))

	m, err := ReadManifest(path)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Empty(t, m.Installs)
}

func TestWriteManifest(t *testing.T) {
	m := &Manifest{Version: 1, Installs: []Install{
		{InstallID: "id1", Entries: []Entry{{ID: "e1", Type: "file_create", Path: "/tmp/x"}}},
	}}
	path := filepath.Join(t.TempDir(), "manifest.json")
	err := WriteManifest(path, m)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), `"install_id": "id1"`)
}

// --- Record tests ---

func TestRecord_Appends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	action := Action{
		Path:            "/tmp/test1",
		Type:            "file_create",
		AfterHash:       "sha256:abc",
		Issue:           "30.4",
		Revertible:      true,
		RevertStrategy:  "delete_file",
	}
	entry, err := Record(path, action)
	require.NoError(t, err)
	require.NotEmpty(t, entry.ID)
	require.Equal(t, "/tmp/test1", entry.Path)

	m, err := ReadManifest(path)
	require.NoError(t, err)
	require.Len(t, m.Installs, 1)
	require.Len(t, m.Installs[0].Entries, 1)
}

func TestRecord_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	_, err := Record(path, Action{Path: "/tmp/a", Type: "file_create", AfterHash: "h1"})
	require.NoError(t, err)

	_, err = Record(path, Action{Path: "/tmp/b", Type: "symlink", AfterHash: "h2"})
	require.NoError(t, err)

	_, err = Record(path, Action{Path: "/tmp/c", Type: "rcfile_append", AfterHash: "h3"})
	require.NoError(t, err)

	m, err := ReadManifest(path)
	require.NoError(t, err)
	require.Len(t, m.Installs, 1)
	require.Len(t, m.Installs[0].Entries, 3)
}

func TestRecord_NewInstallOnNewSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	m := &Manifest{
		Version: 1,
		Installs: []Install{
			{
				InstallID:  "old-install",
				FinishedAt: "yes",
				Entries:    []Entry{{ID: "old-entry", Type: "file_create", Path: "/tmp/old"}},
			},
		},
	}
	require.NoError(t, WriteManifest(path, m))

	entry, err := Record(path, Action{Path: "/tmp/new", Type: "file_create", AfterHash: "h1"})
	require.NoError(t, err)

	m2, err := ReadManifest(path)
	require.NoError(t, err)
	require.Len(t, m2.Installs, 2)
	require.Len(t, m2.Installs[1].Entries, 1)
	require.Equal(t, entry.Path, "/tmp/new")
}

// --- HashFile tests ---

func TestHashFile_Deterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world")
	require.NoError(t, os.WriteFile(path, content, 0o644))

	h1, err := HashFile(path)
	require.NoError(t, err)
	h2, err := HashFile(path)
	require.NoError(t, err)
	require.Equal(t, h1, h2)
}

func TestHashFile_NonExistent(t *testing.T) {
	_, err := HashFile("/nonexistent-file")
	require.Error(t, err)
}

// --- Reversers ---

func TestBlockMarkerReverser_RemovesBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := `source ~/.zshrc.pre

# >>> domain-wrapper >>>
eval "$(domain shell-hook)"
# <<< domain-wrapper <<<

export PATH=$HOME/bin:$PATH
`
	expected := `source ~/.zshrc.pre

export PATH=$HOME/bin:$PATH
`
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))

	r := &BlockMarkerReverser{}
	entry := Entry{
		Path:           path,
		RevertStrategy: "remove_block",
		RevertMetadata: map[string]any{
			"marker_open":  "# >>> domain-wrapper >>>",
			"marker_close": "# <<< domain-wrapper <<<",
		},
	}
	require.True(t, r.CanRevert(entry))

	err := r.Revert(entry)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, expected, string(data))
}

func TestBlockMarkerReverser_NoBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := "no markers here\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))

	r := &BlockMarkerReverser{}
	entry := Entry{
		Path:           path,
		RevertStrategy: "remove_block",
		RevertMetadata: map[string]any{
			"marker_open":  "# >>> domain-wrapper >>>",
			"marker_close": "# <<< domain-wrapper <<<",
		},
	}
	err := r.Revert(entry)
	require.Error(t, err)
}

func TestJSONArrayReverser_RemovesEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := `{
  "hooks": {
    "SessionStart": [
      {"type": "command", "command": "echo hi"},
      {"type": "command", "command": "domain setup auto-detect \"$PWD\" --quiet"}
    ]
  }
}`
	expected := `{
  "hooks": {
    "SessionStart": [
      {"type": "command", "command": "echo hi"}
    ]
  }
}`
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))

	r := &JSONArrayReverser{}
	entry := Entry{
		Path:           path,
		RevertStrategy: "remove_array_entry",
		RevertMetadata: map[string]any{
			"json_key":     "hooks.SessionStart",
			"match_field":  "command",
			"match_prefix": "domain setup auto-detect",
		},
	}
	require.True(t, r.CanRevert(entry))

	err := r.Revert(entry)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, expected, string(data))
}

func TestFileDeleteReverser_DeletesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "some-file")
	require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))

	r := &FileDeleteReverser{}
	entry := Entry{Path: path, RevertStrategy: "delete_file"}
	require.True(t, r.CanRevert(entry))

	err := r.Revert(entry)
	require.NoError(t, err)

	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err))
}

func TestFileDeleteReverser_NotExists(t *testing.T) {
	r := &FileDeleteReverser{}
	entry := Entry{Path: "/nonexistent"}
	err := r.Revert(entry)
	require.NoError(t, err) // idempotent
}

func TestReverserRegistry_Dispatch(t *testing.T) {
	reg := NewReverserRegistry()
	reg.Register("file_delete", &FileDeleteReverser{})

	r, ok := reg.Get("file_delete")
	require.True(t, ok)
	require.NotNil(t, r)

	_, ok = reg.Get("unknown_type")
	require.False(t, ok)
}

func TestUninstall_RequiresConfirm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	_, err := Uninstall(path, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "confirm required")
}

func TestUninstall_DryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	m := &Manifest{
		Version: 1,
		Installs: []Install{
			{InstallID: "inst1", Entries: []Entry{
				{ID: "e1", Type: "file_create", Path: filepath.Join(dir, "f1.txt"), Revertible: true, RevertStrategy: "delete_file", AfterHash: ""},
			}},
		},
	}
	require.NoError(t, WriteManifest(path, m))

	result, err := Uninstall(path, true, true)
	require.NoError(t, err)
	require.Equal(t, 1, result.Reverted)
}

func TestUninstall_RevertsAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	file1 := filepath.Join(dir, "f1.txt")
	require.NoError(t, os.WriteFile(file1, []byte("content1"), 0o644))
	h1, err := HashFile(file1)
	require.NoError(t, err)

	file2 := filepath.Join(dir, "f2.txt")
	require.NoError(t, os.WriteFile(file2, []byte("content2"), 0o644))
	h2, err := HashFile(file2)
	require.NoError(t, err)

	m := &Manifest{
		Version: 1,
		Installs: []Install{
			{InstallID: "inst1", Entries: []Entry{
				{ID: "e1", Type: "file_create", Path: file1, Revertible: true, RevertStrategy: "delete_file", AfterHash: h1},
				{ID: "e2", Type: "file_create", Path: file2, Revertible: true, RevertStrategy: "delete_file", AfterHash: h2},
			}},
		},
	}
	require.NoError(t, WriteManifest(path, m))

	result, err := Uninstall(path, true, false)
	require.NoError(t, err)
	require.Equal(t, 2, result.Reverted)

	_, err = os.Stat(file1)
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(file2)
	require.True(t, os.IsNotExist(err))
}

func TestUninstall_SkipsExternallyModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	file1 := filepath.Join(dir, "f1.txt")
	require.NoError(t, os.WriteFile(file1, []byte("original"), 0o644))
	h1, err := HashFile(file1)
	require.NoError(t, err)

	m := &Manifest{
		Version: 1,
		Installs: []Install{
			{InstallID: "inst1", Entries: []Entry{
				{ID: "e1", Type: "file_create", Path: file1, Revertible: true, RevertStrategy: "delete_file", AfterHash: h1},
			}},
		},
	}
	require.NoError(t, WriteManifest(path, m))

	// Modify the file externally
	require.NoError(t, os.WriteFile(file1, []byte("modified"), 0o644))

	result, err := Uninstall(path, true, false)
	require.NoError(t, err)
	require.Equal(t, 0, result.Reverted)
	require.Equal(t, 1, result.Skipped)
	require.Contains(t, result.Errors[0], "modified externally")

	// File should still exist
	_, err = os.Stat(file1)
	require.NoError(t, err)
}

// --- Sabotage tests ---

func TestSabotage_UninstallNoConfirm(t *testing.T) {
	// This test is the SABOTAGE test for T-sabotaje-1:
	// If we skip the confirm check, uninstall proceeds without prompt.
	// Expected: test_confirm_required would be the one that fails.
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	file1 := filepath.Join(dir, "f1.txt")
	require.NoError(t, os.WriteFile(file1, []byte("content"), 0o644))
	h1, _ := HashFile(file1)

	m := &Manifest{
		Version: 1,
		Installs: []Install{
			{InstallID: "inst1", Entries: []Entry{
				{ID: "e1", Type: "file_create", Path: file1, Revertible: true, RevertStrategy: "delete_file", AfterHash: h1},
			}},
		},
	}
	require.NoError(t, WriteManifest(path, m))

	// Without confirmed=true, uninstall should fail
	_, err := Uninstall(path, false, false)
	require.Error(t, err)
}

func TestSabotage_UninstallSkipsExtModifiedWithoutHashCheck(t *testing.T) {
	// This test verifies the hash check: if we skip hash check,
	// externally modified files get deleted.
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	file1 := filepath.Join(dir, "f1.txt")
	require.NoError(t, os.WriteFile(file1, []byte("original"), 0o644))
	h1, _ := HashFile(file1)

	m := &Manifest{
		Version: 1,
		Installs: []Install{
			{InstallID: "inst1", Entries: []Entry{
				{ID: "e1", Type: "file_create", Path: file1, Revertible: true, RevertStrategy: "delete_file", AfterHash: h1},
			}},
		},
	}
	require.NoError(t, WriteManifest(path, m))

	// Modify the file - hash no longer matches
	require.NoError(t, os.WriteFile(file1, []byte("modified"), 0o644))

	result, err := Uninstall(path, true, false)
	require.NoError(t, err)
	require.Equal(t, 0, result.Reverted, "should skip because hash changed")
	require.Equal(t, 1, result.Skipped)

	// File should still exist
	_, err = os.Stat(file1)
	require.NoError(t, err)
}

func TestReverserRegistry_Multiple(t *testing.T) {
	reg := NewReverserRegistry()
	reg.Register("rcfile_append", &BlockMarkerReverser{})
	reg.Register("claude_settings_merge", &JSONArrayReverser{})
	reg.Register("file_create", &FileDeleteReverser{})
	reg.Register("symlink", &FileDeleteReverser{})

	_, ok := reg.Get("rcfile_append")
	require.True(t, ok)
	_, ok = reg.Get("claude_settings_merge")
	require.True(t, ok)
	_, ok = reg.Get("file_create")
	require.True(t, ok)
	_, ok = reg.Get("symlink")
	require.True(t, ok)
}
