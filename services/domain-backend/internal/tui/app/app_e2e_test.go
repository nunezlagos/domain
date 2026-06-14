// Tests E2E del app model (state machine completa menu ↔ features).
// Movidos desde menu_e2e_test.go (2026-06-12): quit y selección de
// features los resuelve el app model, no el menú standalone.

package app

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func ttyAvailable(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping E2E test in -short mode")
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" || term == "unknown" {
		t.Skip("no TTY (TERM unset/dumb), skipping bubbletea E2E test")
	}
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		t.Skip("stdin is not a TTY, skipping bubbletea E2E test")
	}
}

func TestE2E_App_QuitViaQ(t *testing.T) {
	ttyAvailable(t)
	tm := teatest.NewTestModel(t, New(), teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "DOMAIN")
	}, teatest.WithDuration(10*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
}

func TestE2E_App_QuitViaCtrlC(t *testing.T) {
	ttyAvailable(t)
	tm := teatest.NewTestModel(t, New(), teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "DOMAIN")
	}, teatest.WithDuration(10*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
}

func TestE2E_App_EnterOpensInstallFeature(t *testing.T) {
	ttyAvailable(t)
	tm := teatest.NewTestModel(t, New(), teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Install")
	}, teatest.WithDuration(10*time.Second))

	// Enter en el menú (Install es cursor=0) → feature install activa.
	// La welcome es transitoria (detect platform en ms): lo primero
	// estable que se ve es el prompt de modo con su botón Continuar.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Continuar")
	}, teatest.WithDuration(10*time.Second))

	// ctrl+c desde adentro de la feature DEBE salir (handler global del app).
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
}
