// Tests para internal/tui/selectable (HU-01.13 commit 2/3).

package selectable

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func items3() []Item {
	return []Item{
		{Label: "local", Description: "docker compose"},
		{Label: "cloud", Description: "bring your own"},
		{Label: "hybrid", Description: "per-service", Disabled: true},
	}
}

func TestNew_DefaultsToFirst(t *testing.T) {
	m := New("Mode", items3())
	require.Equal(t, 0, m.cursor)
	require.Equal(t, -1, m.selected)
	require.Equal(t, "Mode", m.title)
}

func TestUpdate_DownWraps(t *testing.T) {
	m := New("x", items3())
	m.cursor = 2
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	concrete := m2.(Model)
	require.Equal(t, 0, concrete.cursor, "down at bottom should wrap to 0")
}

func TestUpdate_UpWraps(t *testing.T) {
	m := New("x", items3())
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	concrete := m2.(Model)
	require.Equal(t, 2, concrete.cursor, "up at top should wrap to last")
}

func TestUpdate_EnterSelectsItem(t *testing.T) {
	m := New("x", items3())
	m.cursor = 1
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	sel, ok := msg.(SelectMsg)
	require.True(t, ok)
	require.Equal(t, 1, sel.Index)
}

func TestUpdate_EnterOnDisabledItemDoesNothing(t *testing.T) {
	m := New("x", items3())
	m.cursor = 2 // hybrid, disabled
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd, "enter on disabled item should be a no-op (no cmd)")
}

func TestUpdate_EscSendsCancel(t *testing.T) {
	m := New("x", items3())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(CancelMsg)
	require.True(t, ok)
}

func TestView_ContainsAllLabels(t *testing.T) {
	m := New("Deployment mode", items3())
	view := m.View()
	require.Contains(t, view, "local")
	require.Contains(t, view, "cloud")
	require.Contains(t, view, "hybrid")
}

func TestView_ContainsCheckboxes(t *testing.T) {
	m := New("x", items3())
	view := m.View()
	require.Contains(t, view, "[ ]")
	require.Contains(t, view, "[-]") // hybrid disabled
}

func TestView_CursorVisible(t *testing.T) {
	m := New("x", items3())
	m.cursor = 1
	view := m.View()
	require.Contains(t, view, ">") // cursor marker
	require.Contains(t, view, "cloud")
}

func TestView_Descriptions(t *testing.T) {
	m := New("x", items3())
	view := m.View()
	require.Contains(t, view, "docker compose")
}

func TestItems_ReturnsDefensiveCopy(t *testing.T) {
	m := New("x", items3())
	items := m.Items()
	items[0].Label = "MUTATED"
	items2 := m.Items()
	require.Equal(t, "local", items2[0].Label, "Items() must return defensive copy")
}

func TestSelectedIndex_InitiallyNegative(t *testing.T) {
	m := New("x", items3())
	require.Equal(t, -1, m.SelectedIndex())
}
