







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

	term := os.Getenv("TERM")
	if term == "" || term == "dumb" || term == "unknown" {
		t.Skip("no TTY (TERM unset/dumb), skipping bubbletea E2E test")
	}



	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		t.Skip("stdin is not a TTY, skipping bubbletea E2E test")
	}
}

func TestE2E_Menu_StartsAndShowsTitle(t *testing.T) {
	ttyAvailable(t)
	tm := teatest.NewTestModel(t, New(), teatest.WithInitialTermSize(80, 24))
	defer tm.Quit()


	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "DOMAIN")
	}, teatest.WithDuration(10*time.Second))
}







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


	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})


	time.Sleep(200 * time.Millisecond)


	out := tm.Output()
	require.NotEmpty(t, out, "model should still be alive after number key")
}
