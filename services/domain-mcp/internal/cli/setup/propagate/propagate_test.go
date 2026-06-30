package propagate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)



func TestScan_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	infos, err := Scan(dir)
	require.NoError(t, err)
	require.Empty(t, infos)
}

func TestScan_WithDomainManifest(t *testing.T) {
	dir := t.TempDir()
	proj1 := filepath.Join(dir, "proj1")
	proj2 := filepath.Join(dir, "proj2")
	proj3 := filepath.Join(dir, "proj3")
	require.NoError(t, os.MkdirAll(filepath.Join(proj1, ".domain"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(proj1, ".domain", "install-manifest.json"), []byte(`{}`), 0o644))
	require.NoError(t, os.MkdirAll(proj2, 0o755))
	require.NoError(t, os.MkdirAll(proj3, 0o755))

	infos, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, infos, 3)

	var found int
	for _, info := range infos {
		if info.HasDomain {
			found++
			require.Equal(t, "proj1", info.Name)
			require.Contains(t, info.DomainManifestAt, "install-manifest.json")
		}
	}
	require.Equal(t, 1, found)
}

func TestScan_DetectsIAConfigs(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "myapp")
	require.NoError(t, os.MkdirAll(proj, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(proj, ".mcp.json"), []byte(`{}`), 0o644))

	infos, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, infos, 1)

	info := infos[0]
	require.ElementsMatch(t, []string{"opencode.json", ".mcp.json"}, info.IAConfigs)
}

func TestScan_OneLevelOnly(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "level1")
	nested := filepath.Join(proj, "level2", "level3")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "opencode.json"), []byte(`{}`), 0o644))

	infos, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, "level1", infos[0].Name)
}

func TestScan_NonExistentPath(t *testing.T) {
	_, err := Scan("/nonexistent-path-12345")
	require.Error(t, err)
}

func TestScan_PathIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("content"), 0o644))
	_, err := Scan(file)
	require.Error(t, err)
}

func TestScan_IsReadOnly(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "myapp")
	require.NoError(t, os.MkdirAll(proj, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{}`), 0o644))


	origInfo, err := os.Stat(proj)
	require.NoError(t, err)
	origMtime := origInfo.ModTime()

	_, err = Scan(dir)
	require.NoError(t, err)


	newInfo, err := os.Stat(proj)
	require.NoError(t, err)
	require.True(t, newInfo.ModTime().Equal(origMtime), "scan should not modify anything")
}



func TestFormatTable_Empty(t *testing.T) {
	out := FormatTable(nil)
	require.Contains(t, out, "NAME")
	require.Contains(t, out, "PATH")
	require.Contains(t, out, "DOMAIN")
	require.Contains(t, out, "IA_CONFIGS")
}

func TestFormatTable_WithProjects(t *testing.T) {
	infos := []ProjectInfo{
		{Name: "proj1", Path: "/tmp/proj1", HasDomain: true, DomainManifestAt: "/tmp/proj1/.domain/install-manifest.json"},
		{Name: "proj2", Path: "/tmp/proj2", HasDomain: false, IAConfigs: []string{"opencode.json", ".mcp.json"}},
		{Name: "proj3", Path: "/tmp/proj3", HasDomain: false, IAConfigs: nil},
	}
	out := FormatTable(infos)
	require.Contains(t, out, "proj1")
	require.Contains(t, out, "proj2")
	require.Contains(t, out, "proj3")
	require.Contains(t, out, "yes")
	require.Contains(t, out, "no")
}



func TestPropagate_SelectNone(t *testing.T) {
	infos := []ProjectInfo{
		{Name: "proj1", Path: "/tmp/proj1"},
	}
	success, failed, errs := Propagate(infos, true)
	require.Equal(t, 0, success)
	require.Equal(t, 0, failed)
	require.Empty(t, errs)
}

func TestPropagate_ContinuesOnFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("requires integration (domain binary)")
	}
	infos := []ProjectInfo{
		{Name: "projA", Path: "/tmp/nonexistent-A-" + t.Name()},
		{Name: "projB", Path: "/tmp/nonexistent-B-" + t.Name()},
	}
	success, failed, errs := Propagate(infos, false)
	require.Equal(t, 0, success)
	require.Equal(t, 2, failed)
	require.Len(t, errs, 2)
}



func TestLoadPropagatePaths_Default(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	paths, err := LoadPropagatePaths()
	require.NoError(t, err)
	require.Equal(t, []string{"~/Proyectos"}, paths)
}

func TestLoadPropagatePaths_Custom(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	configDir := filepath.Join(dir, ".config", "domain")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "propagate-paths.json"), []byte(`{"paths": ["~/work", "~/dev"]}`), 0o644))

	paths, err := LoadPropagatePaths()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"~/work", "~/dev"}, paths)
}
