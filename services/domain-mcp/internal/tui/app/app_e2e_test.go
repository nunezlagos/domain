



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




	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Continuar")
	}, teatest.WithDuration(10*time.Second))


	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
}
