// Tests para internal/tui/features/backups (HU-01.11 commit 4/5).

package backups

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/tui/menu"
)

func TestNew_StartsAtList(t *testing.T) {
	m := New()
	require.Equal(t, stateList, m.state)
	require.Empty(t, m.backups)
}

func TestInit_LoadsBackups(t *testing.T) {
	dir := t.TempDir()
	m := New()
	m.listFn = func() ([]backupEntry, error) {
		return []backupEntry{
			{Path: filepath.Join(dir, "creds.json.bak.20260611T120000Z"), Original: "creds.json"},
		}, nil
	}
	cmd := m.Init()
	require.NotNil(t, cmd)
	msg := cmd()
	res, ok := msg.(listResultMsg)
	require.True(t, ok)
	require.Len(t, res.entries, 1)
}

func TestUpdate_ListResultMsg_StoresAndSorts(t *testing.T) {
	m := New()
	_, _ = m.Update(listResultMsg{entries: []backupEntry{
		{Path: "/b.bak.20260612T120000Z"},
		{Path: "/a.bak.20260611T120000Z"},
	}})
	require.Len(t, m.backups, 2)
	// Sorted: a primero, b despues
	require.Equal(t, "/a.bak.20260611T120000Z", m.backups[0].Path)
}

func TestUpdate_DownMovesCursor(t *testing.T) {
	m := New()
	m.backups = []backupEntry{{Path: "a"}, {Path: "b"}, {Path: "c"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	appM := updated.(*Model)
	require.Equal(t, 1, appM.cursor)
}

func TestUpdate_EnterSelectsItem(t *testing.T) {
	m := New()
	m.backups = []backupEntry{{Path: "a"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.Equal(t, stateAction, appM.state)
	require.NotNil(t, appM.selected)
}

func TestUpdate_EscReturnsToMenu(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	msg := cmd()
	_, isBack := msg.(menu.BackMsg)
	require.True(t, isBack)
}

func TestUpdate_ActionRestore_InvokesRunner(t *testing.T) {
	m := New()
	m.selected = &backupEntry{Path: "/tmp/creds.json.bak.20260611T120000Z"}
	m.state = stateAction
	called := false
	m.runner = func(ctx context.Context, args []string) error {
		called = true
		require.Equal(t, []string{"restore", "/tmp/creds.json.bak.20260611T120000Z"}, args)
		return nil
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	appM := updated.(*Model)
	require.Equal(t, stateRunning, appM.state)
	msg := cmd()
	res := msg.(runResultMsg)
	require.NoError(t, res.err)
	require.True(t, called)
	_ = appM
}

func TestUpdate_ActionDelete_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	backupPath := filepath.Join(dir, "test.bak.20260611T120000Z")
	require.NoError(t, os.WriteFile(backupPath, []byte("data"), 0o600))

	m := New()
	m.selected = &backupEntry{Path: backupPath}
	m.state = stateAction
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	msg := cmd()
	res := msg.(runResultMsg)
	require.NoError(t, res.err)
	// File debe estar borrado
	_, err := os.Stat(backupPath)
	require.True(t, os.IsNotExist(err))
}

func TestUpdate_ActionEsc_BackToList(t *testing.T) {
	m := New()
	m.selected = &backupEntry{Path: "a"}
	m.state = stateAction
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	appM := updated.(*Model)
	require.Equal(t, stateList, appM.state)
	require.Nil(t, appM.selected)
}

func TestUpdate_DoneAnyKeyReturnsToList(t *testing.T) {
	m := New()
	m.state = stateDone
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.Equal(t, stateList, appM.state)
}

func TestUpdate_RunResult_AdvancesToDone(t *testing.T) {
	m := New()
	m.state = stateRunning
	updated, _ := m.Update(runResultMsg{err: nil})
	appM := updated.(*Model)
	require.Equal(t, stateDone, appM.state)
}

func TestView_ListEmpty(t *testing.T) {
	m := New()
	view := m.View()
	require.Contains(t, view, "no backups found")
}

func TestView_ListWithEntries(t *testing.T) {
	m := New()
	m.backups = []backupEntry{{Path: "/tmp/a.bak.X"}}
	view := m.View()
	require.Contains(t, view, "/tmp/a.bak.X")
}

func TestView_ActionMenu(t *testing.T) {
	m := New()
	m.selected = &backupEntry{Path: "/tmp/a.bak.X"}
	m.state = stateAction
	view := m.View()
	require.Contains(t, view, "restore")
	require.Contains(t, view, "delete")
}

func TestView_DoneSuccess(t *testing.T) {
	m := New()
	m.state = stateDone
	m.action = "restore"
	view := m.View()
	require.Contains(t, view, "restore done")
}

func TestView_DoneFailure(t *testing.T) {
	m := New()
	m.state = stateDone
	m.err = os.ErrNotExist
	view := m.View()
	require.Contains(t, view, "Failed")
}
