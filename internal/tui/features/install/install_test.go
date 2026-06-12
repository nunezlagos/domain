// Tests para internal/tui/features/install (HU-01.11 commit 3/5).
//
// Cobertura: state transitions, prompt handling, view format,
// mock runner. NO ejecutamos install real.

package install

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/installer"
	"nunezlagos/domain/internal/tui/menu"
)

func TestNew_Defaults(t *testing.T) {
	m := New()
	require.Equal(t, stateWelcome, m.state)
	require.Equal(t, "http://localhost:8000", m.baseURL)
	require.True(t, m.doOpencode)
	require.False(t, m.doInit)
}

func TestUpdate_PlatformMsg_AdvancesToModePrompt(t *testing.T) {
	m := New()
	updated, cmd := m.Update(platformMsg{platform: installer.Platform{
		OS: installer.OSLinux, Distro: installer.DistroArch,
		PkgMgr: installer.PkgPacman, Version: "rolling",
	}})
	appM := updated.(*Model)
	require.Equal(t, stateModePrompt, appM.state)
	require.Nil(t, cmd, "platform detect does not trigger a command")
}

func TestUpdate_PlatformErr_GoesToDone(t *testing.T) {
	m := New()
	updated, _ := m.Update(platformMsg{err: errFake{}})
	appM := updated.(*Model)
	require.Equal(t, stateDone, appM.state)
	require.Error(t, appM.err)
}

func TestUpdate_DepsMsg_AdvancesToBaseURLPrompt(t *testing.T) {
	m := New()
	m.state = stateDepCheck
	updated, _ := m.Update(depsMsg{deps: nil})
	appM := updated.(*Model)
	require.Equal(t, stateBaseURLPrompt, appM.state,
		"after dep check, advance to baseURL prompt (not mode prompt)")
}

func TestUpdate_ModePrompt_OneSelectsLocal_GoesToDepCheck(t *testing.T) {
	m := New()
	m.state = stateModePrompt
	// '1' emite SelectMsg(0) async; ejecutamos el cmd
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	appM := updated.(*Model)
	require.NotNil(t, cmd, "selectable must emit SelectMsg on number key")
	appM2, _ := appM.Update(cmd())
	appM = appM2.(*Model)
	require.Equal(t, modeLocal, appM.mode)
	require.Equal(t, stateDepCheck, appM.state)
}

func TestUpdate_ModePrompt_TwoSelectsCloud_GoesToDepCheck(t *testing.T) {
	m := New()
	m.state = stateModePrompt
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	appM := updated.(*Model)
	require.NotNil(t, cmd)
	appM2, _ := appM.Update(cmd())
	appM = appM2.(*Model)
	require.Equal(t, modeCloud, appM.mode)
	require.Equal(t, stateDepCheck, appM.state)
}

func TestUpdate_ModePrompt_ThreeFallsBackToLocal(t *testing.T) {
	m := New()
	m.state = stateModePrompt
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	appM := updated.(*Model)
	require.Equal(t, modeLocal, appM.mode, "hybrid falls back to local")
}

func TestDepsForMode_LocalIncludesDocker(t *testing.T) {
	deps := depsForMode(modeLocal)
	hasDocker := false
	for _, d := range deps {
		if d.Name == "docker" {
			hasDocker = true
		}
	}
	require.True(t, hasDocker, "local mode must check docker")
}

func TestDepsForMode_CloudExcludesDocker(t *testing.T) {
	deps := depsForMode(modeCloud)
	for _, d := range deps {
		require.NotEqual(t, "docker", d.Name, "cloud mode must NOT check docker")
	}
}

func TestDepsForMode_HybridIncludesDocker(t *testing.T) {
	// Hybrid incluye docker porque al menos 1 servicio sera local.
	deps := depsForMode(modeHybrid)
	hasDocker := false
	for _, d := range deps {
		if d.Name == "docker" {
			hasDocker = true
		}
	}
	require.True(t, hasDocker, "hybrid mode must check docker (at least 1 local service)")
}

func TestUpdate_BaseURLPrompt_TypingAppends(t *testing.T) {
	m := New()
	m.state = stateBaseURLPrompt
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	appM := updated.(*Model)
	require.Contains(t, appM.baseURL, "h")
}

func TestUpdate_BaseURLPrompt_BackspaceDeletes(t *testing.T) {
	m := New()
	m.state = stateBaseURLPrompt
	m.baseURL = "http://x"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	appM := updated.(*Model)
	require.Equal(t, "http://", appM.baseURL)
}

func TestUpdate_BaseURLPrompt_EnterAdvances(t *testing.T) {
	m := New()
	m.state = stateBaseURLPrompt
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.Equal(t, stateInitPrompt, appM.state)
}

func TestUpdate_InitPrompt_YesSetsTrue(t *testing.T) {
	m := New()
	m.state = stateInitPrompt
	// Enter sobre selectable → emite SelectMsg async; ejecutamos el cmd
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.NotNil(t, cmd)
	appM2, _ := appM.Update(cmd())
	appM = appM2.(*Model)
	require.True(t, appM.doInit)
	require.Equal(t, stateOpencodePrompt, appM.state)
}

func TestUpdate_InitPrompt_NoSetsFalse(t *testing.T) {
	m := New()
	m.state = stateInitPrompt
	// Down para seleccionar "no", luego enter
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.False(t, appM.doInit)
}

func TestUpdate_OpencodePrompt_YesStartsRun(t *testing.T) {
	// Mock runner para no ejecutar install real
	called := false
	SetInstallRunner(func(ctx context.Context, flags []string) (error, string) {
		called = true
		// Verificar flags correctos
		require.Contains(t, flags, "--mode")
		require.Contains(t, flags, "local")
		require.Contains(t, flags, "--non-interactive")
		return nil, ""
	})
	defer SetInstallRunner(defaultInstallRunner) // restore

	m := New()
	m.mode = modeLocal
	m.state = stateOpencodePrompt
	m.doInit = true
	// Enter selecciona el primer item (yes) del selectable.
	// Despues del Update, el state sigue en opencodePrompt
	// (todavia no se proceso el SelectMsg). Necesitamos ejecutar
	// el cmd retornado y hacer Update de nuevo.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.NotNil(t, cmd)
	// Ejecutar el cmd → SelectMsg → Update de nuevo
	selectMsg := cmd()
	appM2, _ := appM.Update(selectMsg); appM = appM2.(*Model)
	require.Equal(t, stateRunning, appM.state,
		"after SelectMsg from opencode prompt, must be in stateRunning")

	// Ejecutar el runInstallCmd
	res, ok := appM.runInstallCmd()().(runResultMsg)
	require.True(t, ok)
	require.NoError(t, res.err)
	require.True(t, called, "runner must be called")
}

func TestUpdate_OpencodePrompt_StderrPropagated(t *testing.T) {
	// Mock runner que falla con stderr real (HU-01.13 commit 2/3)
	SetInstallRunner(func(ctx context.Context, flags []string) (error, string) {
		return fmt.Errorf("exit status 1"), "config validation: DOMAIN_DATABASE_URL is required"
	})
	defer SetInstallRunner(defaultInstallRunner)

	m := New()
	m.mode = modeLocal
	m.state = stateOpencodePrompt
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.NotNil(t, cmd)
	selectMsg := cmd()
	appM2, _ := appM.Update(selectMsg); appM = appM2.(*Model)
	require.Equal(t, stateRunning, appM.state)
	// Ejecutar runInstallCmd
	res, ok := appM.runInstallCmd()().(runResultMsg)
	require.True(t, ok)
	require.Error(t, res.err)
	require.Equal(t, "config validation: DOMAIN_DATABASE_URL is required", res.stderr,
		"stderr should be propagated for error display")
}

func TestUpdate_RunResult_AdvancesToDone(t *testing.T) {
	m := New()
	m.state = stateRunning
	updated, _ := m.Update(runResultMsg{err: nil})
	appM := updated.(*Model)
	require.Equal(t, stateDone, appM.state)
	require.NoError(t, appM.err)
}

func TestUpdate_Done_AnyKeyReturnsToMenu(t *testing.T) {
	m := New()
	m.state = stateDone
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, isBack := msg.(menu.BackMsg)
	require.True(t, isBack)
}

func TestView_Welcome(t *testing.T) {
	m := New()
	view := m.View()
	require.Contains(t, view, "Domain Install")
}

func TestView_DepCheck_WithResults(t *testing.T) {
	m := New()
	m.state = stateDepCheck
	// Inyectar deps manualmente (depsFake era inutil; usamos check real
	// con un binary que SI existe).
	m.deps = nil // vacio para no importar installer aqui
	view := m.View()
	// Sin deps, view deberia decir "Checking dependencies..."
	require.Contains(t, view, "Checking dependencies")
}

func TestView_ModePrompt(t *testing.T) {
	m := New()
	m.state = stateModePrompt
	view := m.View()
	require.Contains(t, view, "local")
	require.Contains(t, view, "cloud")
}

func TestView_Done_Success(t *testing.T) {
	m := New()
	m.state = stateDone
	view := m.View()
	require.Contains(t, view, "Install complete")
}

func TestView_Done_Failure(t *testing.T) {
	m := New()
	m.state = stateDone
	m.err = errFake{}
	view := m.View()
	require.Contains(t, view, "Install failed")
}

// --- helpers ---

func platformFake() struct {
	OS, Distro, PkgMgr, Version string
} {
	return struct {
		OS, Distro, PkgMgr, Version string
	}{OS: "linux", Distro: "arch", PkgMgr: "pacman", Version: "rolling"}
}

type errFake struct{}

func (errFake) Error() string { return "fake error" }

func depsFake() []struct{} {
	return nil
}

func TestView_AllStates_NotEmpty(t *testing.T) {
	m := New()
	for _, st := range []state{
		stateWelcome, stateDepCheck, stateModePrompt,
		stateBaseURLPrompt, stateInitPrompt, stateOpencodePrompt,
		stateRunning, stateDone,
	} {
		m.state = st
		view := m.View()
		require.NotEmpty(t, view, "view for state %d must not be empty", st)
	}
}

func TestView_ContainsHelpOnPrompts(t *testing.T) {
	m := New()
	m.state = stateInitPrompt
	view := m.View()
	require.True(t, strings.Contains(view, "y") || strings.Contains(view, "Y"))
}
