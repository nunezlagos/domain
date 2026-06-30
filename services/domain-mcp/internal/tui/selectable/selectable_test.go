


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

func key(k tea.KeyType) tea.KeyMsg  { return tea.KeyMsg{Type: k} }
func rune1(r rune) tea.KeyMsg       { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestNew_PreselectsFirstEnabled(t *testing.T) {
	m := New("Mode", items3())
	require.Equal(t, 0, m.cursor)
	require.Equal(t, 0, m.selected, "primer item habilitado pre-seleccionado")
	require.Equal(t, "Mode", m.title)
}

func TestUpdate_DownNavigatesIncludingContinue(t *testing.T) {
	m := New("x", items3())
	m.cursor = 2
	m2, _ := m.Update(key(tea.KeyDown))
	concrete := m2.(Model)
	require.Equal(t, 3, concrete.cursor, "down desde el último item va al botón Continuar")

	m3, _ := concrete.Update(key(tea.KeyDown))
	require.Equal(t, 0, m3.(Model).cursor, "down en Continuar wrappea a 0")
}

func TestUpdate_UpWrapsToContinue(t *testing.T) {
	m := New("x", items3())
	m2, _ := m.Update(key(tea.KeyUp))
	require.Equal(t, 3, m2.(Model).cursor, "up en el tope wrappea al botón Continuar")
}

func TestUpdate_EnterOnItem_SelectsAndAdvances(t *testing.T) {

	m := New("x", items3())
	m.cursor = 1
	m2, cmd := m.Update(key(tea.KeyEnter))
	require.NotNil(t, cmd, "enter confirma y avanza")
	require.Equal(t, 1, m2.(Model).selected)
	sel, ok := cmd().(SelectMsg)
	require.True(t, ok)
	require.Equal(t, 1, sel.Index)
}

func TestUpdate_SpaceOnItem_Selects(t *testing.T) {
	m := New("x", items3())
	m.cursor = 1
	m2, cmd := m.Update(key(tea.KeySpace))
	require.Nil(t, cmd)
	require.Equal(t, 1, m2.(Model).selected)
}

func TestUpdate_EnterOnContinue_EmitsSelectMsg(t *testing.T) {
	m := New("x", items3())
	m.cursor = 1
	m2, _ := m.Update(key(tea.KeyEnter)) // elige cloud
	concrete := m2.(Model)
	concrete.cursor = concrete.continueSlot()
	_, cmd := concrete.Update(key(tea.KeyEnter))
	require.NotNil(t, cmd)
	sel, ok := cmd().(SelectMsg)
	require.True(t, ok)
	require.Equal(t, 1, sel.Index)
}

func TestUpdate_TabJumpsToContinue(t *testing.T) {
	m := New("x", items3())
	m2, _ := m.Update(key(tea.KeyTab))
	require.Equal(t, 3, m2.(Model).cursor)
}

func TestUpdate_EnterOnDisabledItemDoesNotSelect(t *testing.T) {
	m := New("x", items3())
	m.cursor = 2 // hybrid, disabled
	m2, cmd := m.Update(key(tea.KeyEnter))
	require.Nil(t, cmd)
	require.Equal(t, 0, m2.(Model).selected, "selección previa intacta")
}

func TestUpdate_NumberKeySelectsWithoutAdvancing(t *testing.T) {
	m := New("x", items3())
	m2, cmd := m.Update(rune1('2'))
	require.Nil(t, cmd, "number key elige pero no avanza")
	concrete := m2.(Model)
	require.Equal(t, 1, concrete.selected)
	require.Equal(t, 1, concrete.cursor)
}

func TestUpdate_EscSendsCancel(t *testing.T) {
	m := New("x", items3())
	_, cmd := m.Update(key(tea.KeyEsc))
	require.NotNil(t, cmd)
	_, ok := cmd().(CancelMsg)
	require.True(t, ok)
}



func TestMulti_SpaceToggles_EnterConfirms(t *testing.T) {
	m := NewMulti("Agentes", items3(), []int{0})
	require.Equal(t, []int{0}, m.CheckedIndices())


	m2, cmd := m.Update(key(tea.KeySpace))
	require.Nil(t, cmd)
	require.Empty(t, m2.(Model).CheckedIndices())


	m3, _ := m2.Update(key(tea.KeyDown))
	m4, cmd := m3.Update(key(tea.KeySpace))
	require.Nil(t, cmd)
	require.Equal(t, []int{1}, m4.(Model).CheckedIndices())

	_, cmd = m4.Update(key(tea.KeyEnter))
	require.NotNil(t, cmd, "enter en multi confirma lo marcado y avanza")
	msg, ok := cmd().(MultiSelectMsg)
	require.True(t, ok)
	require.Equal(t, []int{1}, msg.Indices)
}

func TestMulti_ContinueEmitsMultiSelectMsg(t *testing.T) {
	m := NewMulti("Agentes", items3(), []int{0, 1})
	m.cursor = m.continueSlot()
	_, cmd := m.Update(key(tea.KeyEnter))
	require.NotNil(t, cmd)
	msg, ok := cmd().(MultiSelectMsg)
	require.True(t, ok)
	require.Equal(t, []int{0, 1}, msg.Indices)
}

func TestMulti_ContinueWithEmptySelectionAllowed(t *testing.T) {
	m := NewMulti("Agentes", items3(), nil)
	m.cursor = m.continueSlot()
	_, cmd := m.Update(key(tea.KeyEnter))
	require.NotNil(t, cmd)
	msg := cmd().(MultiSelectMsg)
	require.Empty(t, msg.Indices)
}

func TestMulti_DisabledNotToggleable(t *testing.T) {
	m := NewMulti("x", items3(), nil)
	m.cursor = 2
	m2, _ := m.Update(key(tea.KeySpace))
	require.Empty(t, m2.(Model).CheckedIndices())
}



func TestView_ContainsAllLabelsAndContinue(t *testing.T) {
	m := New("Deployment mode", items3())
	view := m.View()
	require.Contains(t, view, "local")
	require.Contains(t, view, "cloud")
	require.Contains(t, view, "hybrid")
	require.Contains(t, view, "Continuar")
}

func TestView_SingleUsesRadio_MultiUsesCheckbox(t *testing.T) {
	single := New("x", items3())
	require.Contains(t, single.View(), "(•)", "single: item elegido con radio")

	multi := NewMulti("x", items3(), []int{0})
	v := multi.View()
	require.Contains(t, v, "[X]")
	require.Contains(t, v, "[ ]")
	require.Contains(t, v, "[-]") // disabled
}

func TestView_CursorVisible(t *testing.T) {
	m := New("x", items3())
	m.cursor = 1
	view := m.View()
	require.Contains(t, view, ">")
	require.Contains(t, view, "cloud")
}

func TestItems_ReturnsDefensiveCopy(t *testing.T) {
	m := New("x", items3())
	items := m.Items()
	items[0].Label = "MUTATED"
	require.Equal(t, "local", m.Items()[0].Label)
}

// Sabotaje: confirmar sin selección en single NO emite (no se puede
// avanzar sin elegir).
func TestSabotage_Single_NoSelectionNoAdvance(t *testing.T) {
	allDisabled := []Item{{Label: "a", Disabled: true}}
	m := New("x", allDisabled)
	require.Equal(t, -1, m.selected)
	m.cursor = m.continueSlot()
	_, cmd := m.Update(key(tea.KeyEnter))
	require.Nil(t, cmd, "Continuar sin selección debe ser no-op")
}
