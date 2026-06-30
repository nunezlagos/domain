






package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"nunezlagos/domain/internal/tui/menu"
	"nunezlagos/domain/internal/tui/styles"
)

type state int

const (
	stateConfirm state = iota
	stateRunning
	stateDone
)

type runResultMsg struct {
	err error
}

// Model bubbletea para la feature update.
type Model struct {
	state   state
	confirm bool // user ya confirmo
	err     error

	runner func(ctx context.Context, args []string) error
}

func New() *Model {
	return &Model{state: stateConfirm, runner: defaultUpdateRunner}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runResultMsg:
		m.err = msg.err
		m.state = stateDone
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case stateConfirm:
			switch msg.String() {
			case "enter", "y", "Y":
				m.confirm = true
				m.state = stateRunning
				return m, m.runUpdateCmd()
			case "esc", "q", "n", "N":
				return m, backCmd()
			}
		case stateRunning:

		case stateDone:

			return m, backCmd()
		}
	}
	return m, nil
}

func (m *Model) View() string {
	switch m.state {
	case stateConfirm:
		s := "\n"
		s += styles.Title.Render("  Domain Update") + "\n"
		s += "\n"
		s += "  This will run:\n"
		s += "    1. Backup credentials, .env, opencode.json (timestamped)\n"
		s += "    2. Apply pending migrations\n"
		s += "    3. Run seeders (idempotent skip-by-hash)\n"
		s += "\n"
		s += "  Press [enter] to run, [esc] to go back\n"
		return s
	case stateRunning:
		return "\n  Running update (see output below)...\n"
	case stateDone:
		s := "\n"
		if m.err != nil {
			s += styles.Fail.Render("  Update failed: ") + m.err.Error() + "\n"
		} else {
			s += styles.Ok.Render("  Update complete.") + "\n"
		}
		s += "\n  Press any key to return to menu.\n"
		return s
	}
	return ""
}

func (m *Model) runUpdateCmd() tea.Cmd {
	runner := m.runner
	return func() tea.Msg {
		err := runner(context.Background(), []string{"update"})
		return runResultMsg{err: err}
	}
}

func backCmd() tea.Cmd {
	return func() tea.Msg { return menu.BackMsg{} }
}

func defaultUpdateRunner(ctx context.Context, args []string) error {
	bin, err := exec.LookPath("domain")
	if err != nil {
		if _, statErr := os.Stat("./bin/domain"); statErr == nil {
			bin, _ = filepath.Abs("./bin/domain")
		} else {
			return fmt.Errorf("domain binary not found: %w", err)
		}
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
