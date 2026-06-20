// Tests para internal/tui/app (HU-01.11 commit 2/5).
//
// Cobertura: state transitions, menu → feature, feature → menu (BackMsg),
// exit, view switch.

package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/tui/menu"
)

func TestNew_StartsInMenu(t *testing.T) {
	m := New()
	view := m.View()
	require.Contains(t, view, "DOMAIN")
	require.Contains(t, view, "Install")
}

func TestUpdate_SelectInstall_GoesToFeature(t *testing.T) {
	m := New()
	updated, _ := m.Update(menu.SelectMsg{Index: menu.IndexInstall})
	appM, ok := updated.(Model)
	require.True(t, ok)
	// View ya no es el menu
	view := appM.View()
	require.NotContains(t, view, "[1/4]")
}

func TestUpdate_SelectExit_Quits(t *testing.T) {
	m := New()
	_, cmd := m.Update(menu.SelectMsg{Index: menu.IndexExit})
	require.NotNil(t, cmd)
	// Ejecutamos el cmd y verificamos que es tea.QuitMsg
	msg := cmd()
	_, isQuit := msg.(tea.QuitMsg)
	require.True(t, isQuit, "selecting Exit should send tea.QuitMsg")
}

func TestUpdate_BackMsg_ReturnsToMenu(t *testing.T) {
	m := New()
	// Forzamos entrar en una feature
	updated, _ := m.Update(menu.SelectMsg{Index: menu.IndexInstall})
	appM := updated.(Model)
	// Enviamos BackMsg
	updated2, _ := appM.Update(BackMsg{})
	appM2 := updated2.(Model)
	// View deberia ser el menu de nuevo
	view := appM2.View()
	require.True(t, strings.Contains(view, "Install"),
		"after BackMsg, view should be the menu again")
}

func TestNewDirect_Install_StartsInFeature(t *testing.T) {
	m := NewDirect(menu.IndexInstall)
	// View deberia ser de la feature, no el menu
	view := m.View()
	require.NotContains(t, view, "[1/4]")
}

func TestUpdate_UnknownIndex_StaysInMenu(t *testing.T) {
	m := New()
	updated, _ := m.Update(menu.SelectMsg{Index: 99})
	appM := updated.(Model)
	// Sigue en el menu
	view := appM.View()
	require.Contains(t, view, "DOMAIN")
}

func TestFeatureFor_KnownIndexes(t *testing.T) {
	for _, idx := range []int{menu.IndexInstall, menu.IndexUpdate, menu.IndexBackups} {
		require.NotNil(t, featureFor(idx), "feature for index %d must exist", idx)
	}
	require.Nil(t, featureFor(99), "unknown index returns nil")
}
