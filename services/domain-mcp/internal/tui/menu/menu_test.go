






package menu

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestNew_DefaultsToFirstItem(t *testing.T) {
	m := New()
	require.Equal(t, 0, m.cursor)
	require.Len(t, m.items, 4)
}

func TestInit_ReturnsNilCmd(t *testing.T) {
	m := New()
	require.Nil(t, m.Init())
}

func TestUpdate_DownMovesCursor(t *testing.T) {
	m := New()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	concrete := m2.(Model)
	require.Equal(t, 1, concrete.cursor)
}

func TestUpdate_UpAtTopStays(t *testing.T) {
	m := New()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	concrete := m2.(Model)
	require.Equal(t, 0, concrete.cursor, "up at top must not wrap")
}

func TestUpdate_DownAtBottomStays(t *testing.T) {
	m := New()
	m.cursor = 3 // last
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	concrete := m2.(Model)
	require.Equal(t, 3, concrete.cursor, "down at bottom must not wrap")
}

func TestUpdate_EnterSendsSelectMsg(t *testing.T) {
	m := New()
	m.cursor = 1
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	selectMsg, ok := msg.(SelectMsg)
	require.True(t, ok)
	require.Equal(t, 1, selectMsg.Index)
	_ = tea.KeyMsg{}
}

func TestUpdate_QuitKeysSelectExit(t *testing.T) {
	for _, key := range []string{"q", "esc", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			m := New()
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // esc y q testeados por string
			if key == "q" {
				_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
			} else if key == "ctrl+c" {
				_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			}
			require.NotNil(t, cmd)
			msg := cmd()
			selectMsg, ok := msg.(SelectMsg)
			require.True(t, ok, "key %q should send SelectMsg", key)
			require.Equal(t, IndexExit, selectMsg.Index)
		})
	}
}

func TestUpdate_NumberKeysJumpAndSelect(t *testing.T) {
	for n, wantIdx := range map[string]int{"1": 0, "2": 1, "3": 2, "4": 3} {
		t.Run(n, func(t *testing.T) {
			m := New()
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(n)})
			require.NotNil(t, cmd)
			msg := cmd()
			selectMsg, ok := msg.(SelectMsg)
			require.True(t, ok)
			require.Equal(t, wantIdx, selectMsg.Index)
		})
	}
}

func TestView_ContainsAllItemTitles(t *testing.T) {
	m := New()
	view := m.View()
	for _, it := range m.items {
		require.Contains(t, view, it.title, "view must contain item title %q", it.title)
	}
	require.Contains(t, view, "DOMAIN")
	require.Contains(t, view, "enter")
}

func TestIndexOf(t *testing.T) {
	require.Equal(t, IndexInstall, IndexOf("install"))
	require.Equal(t, IndexInstall, IndexOf("Install"))
	require.Equal(t, IndexUpdate, IndexOf("UPDATE"))
	require.Equal(t, -1, IndexOf("nonexistent"))
}

func TestFeatureNames(t *testing.T) {
	names := FeatureNames()
	require.Equal(t, []string{"Install", "Update", "Backups", "Exit"}, names)
}

func TestView_CursorVisible(t *testing.T) {
	m := New()
	m.cursor = 2
	view := m.View()

	lines := strings.Split(view, "\n")
	found := false
	for _, l := range lines {
		if strings.Contains(l, "Backups") && strings.Contains(l, ">") {
			found = true
			break
		}
	}
	require.True(t, found, "selected item should have '>' cursor marker")
}
