// Tests para internal/tui/features/update (HU-01.11 commit 4/5).

package update

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/tui/menu"
)

func TestNew_StartsAtConfirm(t *testing.T) {
	m := New()
	require.Equal(t, stateConfirm, m.state)
	require.False(t, m.confirm)
}

func TestUpdate_EnterStartsRun(t *testing.T) {
	m := New()
	m.runner = func(ctx context.Context, args []string) error {
		require.Equal(t, []string{"update"}, args)
		return nil
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.Equal(t, stateRunning, appM.state)
	require.NotNil(t, cmd)
	msg := cmd()
	res, ok := msg.(runResultMsg)
	require.True(t, ok)
	require.NoError(t, res.err)
}

func TestUpdate_EscReturnsToMenu(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	msg := cmd()
	_, isBack := msg.(menu.BackMsg)
	require.True(t, isBack)
}

func TestUpdate_RunResult_AdvancesToDone(t *testing.T) {
	m := New()
	m.state = stateRunning
	updated, _ := m.Update(runResultMsg{err: nil})
	appM := updated.(*Model)
	require.Equal(t, stateDone, appM.state)
}

func TestUpdate_DoneAnyKeyReturnsToMenu(t *testing.T) {
	m := New()
	m.state = stateDone
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, isBack := msg.(menu.BackMsg)
	require.True(t, isBack)
}

func TestUpdate_RunWithError_Propagates(t *testing.T) {
	m := New()
	m.runner = func(ctx context.Context, args []string) error {
		return context.Canceled
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	msg := cmd()
	res := msg.(runResultMsg)
	_ = appM
	require.Error(t, res.err)
}

func TestView_AllStates(t *testing.T) {
	m := New()
	for _, st := range []state{stateConfirm, stateRunning, stateDone} {
		m.state = st
		view := m.View()
		require.NotEmpty(t, view)
	}
}

func TestView_ConfirmShowsActions(t *testing.T) {
	m := New()
	view := m.View()
	require.Contains(t, view, "Backup")
	require.Contains(t, view, "migrations")
	require.Contains(t, view, "seeders")
}

func TestView_DoneSuccess(t *testing.T) {
	m := New()
	m.state = stateDone
	view := m.View()
	require.Contains(t, view, "Update complete")
}

func TestView_DoneFailure(t *testing.T) {
	m := New()
	m.state = stateDone
	m.err = context.Canceled
	view := m.View()
	require.Contains(t, view, "Update failed")
}
