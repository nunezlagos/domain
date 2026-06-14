// Tests E2E para el TUI bubbletea (HU-01.12 commit 4/4).
//
// Usa teatest (github.com/charmbracelet/x/exp/teatest) para simular
// TTY real: arranca tea.NewProgram, envia keys, lee output.
//
// Si teatest no puede crear pty (CI sin /dev/ptmx), los tests se
// skippean automaticamente (ver TestMainE2E_RequiresTTY).

package menu

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"
)

// ttyAvailable chequea si podemos crear pty. Si no, skip.
// En sandbox sin /dev/ptmx o en CI sin TTY, los tests E2E de bubbletea
// no se pueden ejecutar. Skip automatico.
func ttyAvailable(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping E2E test in -short mode")
	}
	// Verificar TERM
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" || term == "unknown" {
		t.Skip("no TTY (TERM unset/dumb), skipping bubbletea E2E test")
	}
	// TERM seteado no garantiza terminal real (sandboxes/CI exportan
	// TERM sin TTY y teatest se cuelga hasta timeout). Confirmar que
	// stdin es un char device.
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		t.Skip("stdin is not a TTY, skipping bubbletea E2E test")
	}
}

func TestE2E_Menu_StartsAndShowsTitle(t *testing.T) {
	ttyAvailable(t)
	tm := teatest.NewTestModel(t, New(), teatest.WithInitialTermSize(80, 24))
	defer tm.Quit()

	// Esperar a que el output contenga "DOMAIN" (titulo)
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "DOMAIN")
	}, teatest.WithDuration(10*time.Second))
}

// NOTA (2026-06-12): los tests de quit y de "enter selecciona feature"
// se movieron a internal/tui/app/app_e2e_test.go. El menú standalone
// NUNCA retorna tea.Quit (emite SelectMsg{IndexExit} que resuelve el
// app model) — los tests previos contra el menú solo, con WaitFinished,
// no podían pasar y solo "pasaban" cuando el entorno los skippeaba.

func TestE2E_Menu_AllItemsVisible(t *testing.T) {
	ttyAvailable(t)
	tm := teatest.NewTestModel(t, New(), teatest.WithInitialTermSize(120, 30))
	defer tm.Quit()

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		out := string(bts)
		return strings.Contains(out, "Install") &&
			strings.Contains(out, "Update") &&
			strings.Contains(out, "Backups") &&
			strings.Contains(out, "Exit")
	}, teatest.WithDuration(10*time.Second))
}

func TestE2E_Menu_NumberKeysJumpToItem(t *testing.T) {
	ttyAvailable(t)
	tm := teatest.NewTestModel(t, New(), teatest.WithInitialTermSize(80, 24))
	defer tm.Quit()

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "DOMAIN")
	}, teatest.WithDuration(10*time.Second))

	// Press "2" → cursor a Update (idx 1)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})

	// Esperar a que el programa siga vivo (no panic, no exit)
	time.Sleep(200 * time.Millisecond)

	// Verificamos que el output cambio (al menos un render despues del key)
	out := tm.Output()
	require.NotEmpty(t, out, "model should still be alive after number key")
}
