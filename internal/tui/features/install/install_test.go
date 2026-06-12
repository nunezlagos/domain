// Tests para internal/tui/features/install (rediseño 2026-06-11):
// config completa primero → summary → instalación automática con
// streaming. NO ejecutamos install real (mock runner).

package install

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/installer"
	"nunezlagos/domain/internal/tui/menu"
)

func TestUpdate_Done_AnyKeyReturnsToMenu(t *testing.T) {
	m := New()
	m.state = stateDone
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	_, isBack := cmd().(menu.BackMsg)
	require.True(t, isBack)
}

func TestNew_Defaults(t *testing.T) {
	m := New()
	require.Equal(t, stateWelcome, m.state)
	require.NotEmpty(t, m.port, "puerto sugerido no vacío")
	require.True(t, m.doInit)
	require.Equal(t, []string{"opencode"}, m.agents)
}

func TestSuggestPort_ReturnsNumeric(t *testing.T) {
	p := suggestPort(8000)
	require.Regexp(t, `^\d{4,5}$`, p)
}

func TestUpdate_PlatformMsg_AdvancesToModePrompt(t *testing.T) {
	m := New()
	updated, cmd := m.Update(platformMsg{platform: installer.Platform{
		OS: installer.OSLinux, Distro: installer.DistroArch,
		PkgMgr: installer.PkgPacman, Version: "rolling",
	}})
	appM := updated.(*Model)
	require.Equal(t, stateModePrompt, appM.state)
	require.Nil(t, cmd)
}

func TestUpdate_PlatformErr_GoesToDone(t *testing.T) {
	m := New()
	updated, _ := m.Update(platformMsg{err: errFake{}})
	require.Equal(t, stateDone, updated.(*Model).state)
	require.Error(t, updated.(*Model).err)
}

// selectMode simula: elegir el item idx y confirmar con Continuar.
func selectMode(t *testing.T, m *Model, idx int) *Model {
	t.Helper()
	// number key elige sin avanzar
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('1' + idx)}})
	require.Nil(t, cmd, "number key no avanza")
	appM := updated.(*Model)
	// tab → Continuar, enter → SelectMsg
	updated, _ = appM.Update(tea.KeyMsg{Type: tea.KeyTab})
	appM = updated.(*Model)
	updated, cmd = appM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM = updated.(*Model)
	require.NotNil(t, cmd, "enter en Continuar emite SelectMsg")
	updated, _ = appM.Update(cmd())
	return updated.(*Model)
}

func TestFlow_ModeLocal_RequiresContinueToAdvance(t *testing.T) {
	m := New()
	m.state = stateModePrompt
	appM := selectMode(t, m, 0)
	require.Equal(t, modeLocal, appM.mode)
	require.Equal(t, stateDepCheck, appM.state)
}

func TestFlow_ModeCloud_GoesToDSNAfterDeps(t *testing.T) {
	m := New()
	m.state = stateModePrompt
	appM := selectMode(t, m, 1)
	require.Equal(t, modeCloud, appM.mode)
	require.Equal(t, stateDepCheck, appM.state)

	// deps OK → enter → DSN prompt (cloud)
	updated, _ := appM.Update(depsMsg{deps: nil})
	appM = updated.(*Model)
	updated, _ = appM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, stateDSNPrompt, updated.(*Model).state)
}

func TestDepCheck_MissingDepBlocksContinue(t *testing.T) {
	m := New()
	m.state = stateDepCheck
	updated, _ := m.Update(depsMsg{deps: []installer.CheckResult{
		{Dep: installer.DepDocker, Found: false, Hint: "pacman -S docker"},
	}})
	appM := updated.(*Model)
	require.True(t, appM.depsMissing)
	updated, _ = appM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, stateDepCheck, updated.(*Model).state, "enter bloqueado con deps faltantes")
}

func TestDepCheck_AllOK_AdvancesToPort(t *testing.T) {
	m := New()
	m.mode = modeLocal
	m.state = stateDepCheck
	updated, _ := m.Update(depsMsg{deps: []installer.CheckResult{
		{Dep: installer.DepGo, Found: true, MinMet: true},
	}})
	appM := updated.(*Model)
	require.False(t, appM.depsMissing)
	updated, _ = appM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, statePortPrompt, updated.(*Model).state)
}

func TestPortPrompt_OnlyDigits(t *testing.T) {
	m := New()
	m.state = statePortPrompt
	m.port = ""
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}})
	appM := updated.(*Model)
	require.Equal(t, "9", appM.port)
	// Letra: ignorada
	updated, _ = appM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	require.Equal(t, "9", updated.(*Model).port)
}

func TestPortPrompt_EnterAdvancesToInit(t *testing.T) {
	m := New()
	m.state = statePortPrompt
	m.port = "8123"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.Equal(t, stateInitPrompt, appM.state)
	require.Equal(t, "http://localhost:8123", appM.baseURL())
}

func TestDSNPrompt_EmptyDSNDoesNotAdvance(t *testing.T) {
	m := New()
	m.mode = modeCloud
	m.state = stateDSNPrompt
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, stateDSNPrompt, updated.(*Model).state, "DSN vacía no avanza")
}

func TestInitPrompt_SelectNoViaContinue(t *testing.T) {
	m := New()
	m.state = stateInitPrompt
	// down → "no", espacio elige, tab+enter confirma
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	appM := updated.(*Model)
	updated, _ = appM.Update(tea.KeyMsg{Type: tea.KeySpace})
	appM = updated.(*Model)
	updated, _ = appM.Update(tea.KeyMsg{Type: tea.KeyTab})
	appM = updated.(*Model)
	updated, cmd := appM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM = updated.(*Model)
	require.NotNil(t, cmd)
	updated, _ = appM.Update(cmd())
	appM = updated.(*Model)
	require.False(t, appM.doInit)
	require.Equal(t, stateAgentsPrompt, appM.state)
}

func TestAgentsPrompt_MultiSelectBoth(t *testing.T) {
	m := New()
	m.state = stateAgentsPrompt
	// opencode ya viene marcado; bajar a claude-code y marcarlo
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	appM := updated.(*Model)
	updated, _ = appM.Update(tea.KeyMsg{Type: tea.KeySpace})
	appM = updated.(*Model)
	// confirmar
	updated, _ = appM.Update(tea.KeyMsg{Type: tea.KeyTab})
	appM = updated.(*Model)
	updated, cmd := appM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM = updated.(*Model)
	require.NotNil(t, cmd)
	updated, _ = appM.Update(cmd())
	appM = updated.(*Model)
	require.Equal(t, []string{"opencode", "claude-code"}, appM.agents)
	require.Equal(t, stateSummary, appM.state)
}

func TestSummary_EnterStartsInstallWithFlags(t *testing.T) {
	var gotFlags []string
	SetInstallRunner(func(ctx context.Context, flags []string, onLine func(string)) (error, string) {
		gotFlags = flags
		onLine("[1/9] Detecting state")
		return nil, ""
	})
	defer SetInstallRunner(defaultInstallRunner)

	m := New()
	m.mode = modeLocal
	m.port = "8042"
	m.doInit = false
	m.agents = []string{"opencode", "claude-code"}
	m.state = stateSummary

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	require.Equal(t, stateRunning, appM.state)
	require.NotNil(t, cmd)

	// Drenar el canal directo (el cmd real es un tea.Batch con el tick
	// de animación; en tests leemos los mensajes del runner nomás).
	appM = drainRun(t, appM)
	require.NoError(t, appM.err)
	require.Contains(t, appM.lines, "[1/9] Detecting state", "output streameado visible")

	require.Contains(t, gotFlags, "--mode")
	require.Contains(t, gotFlags, "local")
	require.Contains(t, gotFlags, "--base-url")
	require.Contains(t, gotFlags, "http://localhost:8042")
	require.Contains(t, gotFlags, "--agents")
	require.Contains(t, gotFlags, "opencode,claude-code")
	require.Contains(t, gotFlags, "--no-init")
	require.Contains(t, gotFlags, "--non-interactive")
}

func TestRunning_StderrPropagatedOnFailure(t *testing.T) {
	SetInstallRunner(func(ctx context.Context, flags []string, onLine func(string)) (error, string) {
		onLine("[3/9] Applying migrations")
		return fmt.Errorf("exit status 1"), "config validation: DOMAIN_DATABASE_URL is required"
	})
	defer SetInstallRunner(defaultInstallRunner)

	m := New()
	m.mode = modeLocal
	m.state = stateSummary
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	appM := updated.(*Model)
	appM = drainRun(t, appM)
	require.Error(t, appM.err)
	require.Equal(t, "config validation: DOMAIN_DATABASE_URL is required", appM.stderr)
	view := appM.View()
	require.Contains(t, view, "falló")
	require.Contains(t, view, "DOMAIN_DATABASE_URL")
}

// drainRun consume mensajes del canal del runner hasta llegar a done.
func drainRun(t *testing.T, appM *Model) *Model {
	t.Helper()
	require.NotNil(t, appM.runCh)
	for appM.state != stateDone {
		msg := waitForRunMsg(appM.runCh)()
		updated, _ := appM.Update(msg)
		appM = updated.(*Model)
	}
	return appM
}

func TestRedactDSN(t *testing.T) {
	require.Equal(t, "postgres://app:***@db:5432/x", redactDSN("postgres://app:secret@db:5432/x"))
	require.Equal(t, "postgres://db:5432/x", redactDSN("postgres://db:5432/x"))
}

func TestDepsForMode_LocalIncludesDocker(t *testing.T) {
	deps := depsForMode(modeLocal)
	hasDocker := false
	for _, d := range deps {
		if d.Name == "docker" {
			hasDocker = true
		}
	}
	require.True(t, hasDocker)
}

func TestDepsForMode_CloudExcludesDocker(t *testing.T) {
	for _, d := range depsForMode(modeCloud) {
		require.NotEqual(t, "docker", d.Name)
	}
}

func TestView_AllStates_NotEmpty(t *testing.T) {
	m := New()
	for _, st := range []state{
		stateWelcome, stateModePrompt, stateDepCheck, statePortPrompt,
		stateDSNPrompt, stateInitPrompt, stateAgentsPrompt, stateSummary,
		stateRunning, stateDone,
	} {
		m.state = st
		require.NotEmpty(t, m.View(), "view del state %d vacía", st)
	}
}

func TestView_Summary_ShowsConfig(t *testing.T) {
	m := New()
	m.mode = modeLocal
	m.port = "8005"
	m.agents = []string{"opencode"}
	m.state = stateSummary
	view := m.View()
	require.Contains(t, view, "local")
	require.Contains(t, view, "8005")
	require.Contains(t, view, "opencode")
	require.Contains(t, view, "Instalar")
}

// --- helpers ---

type errFake struct{}

func (errFake) Error() string { return "fake error" }
