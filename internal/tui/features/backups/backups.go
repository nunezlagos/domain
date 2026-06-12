// Package backups — TUI feature para `domain backups` (HU-01.11).
//
// Flow:
//   1. Lista backups disponibles (credentials, .env, opencode.json)
//   2. User navega con flechas
//   3. Acciones: [r] restore, [d] delete, [esc] back
//   4. Restore invoca `domain restore <bak-path>` como sub-process

package backups

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"nunezlagos/domain/internal/cli/install"
	"nunezlagos/domain/internal/tui/menu"
	"nunezlagos/domain/internal/tui/styles"
)

type state int

const (
	stateList state = iota
	stateAction
	stateRunning
	stateDone
)

// backupEntry representa un backup individual en la lista.
type backupEntry struct {
	Path     string // path completo al .bak.<ts>
	Original string // path original (e.g., credentials.json)
	Size     int64
}

// Model bubbletea para la feature backups.
type Model struct {
	state   state
	backups []backupEntry
	cursor  int
	selected *backupEntry // cuando user elige uno
	err     error
	// listFn: tests lo mockean para no leer filesystem real.
	listFn func() ([]backupEntry, error)
	runner func(ctx context.Context, args []string) error
	// action: "restore" | "delete" | ""
	action string
}

func New() *Model {
	return &Model{
		state:   stateList,
		listFn:  listBackups,
		runner:  defaultRestoreRunner,
	}
}

func (m *Model) Init() tea.Cmd {
	// Cargar backups en startup
	return func() tea.Msg {
		entries, err := m.listFn()
		return listResultMsg{entries: entries, err: err}
	}
}

type listResultMsg struct {
	entries []backupEntry
	err     error
}

type runResultMsg struct {
	err error
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case listResultMsg:
		m.backups = msg.entries
		m.err = msg.err
		// Sort por path (lexicografico = timestamp order)
		sort.Slice(m.backups, func(i, j int) bool {
			return m.backups[i].Path < m.backups[j].Path
		})
		return m, nil
	case runResultMsg:
		m.err = msg.err
		m.state = stateDone
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.state {
	case stateList:
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.backups)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.backups) > 0 {
				m.selected = &m.backups[m.cursor]
				m.state = stateAction
			}
		case "esc", "q":
			return m, backCmd()
		}
	case stateAction:
		switch key {
		case "r":
			if m.selected != nil {
				m.action = "restore"
				m.state = stateRunning
				return m, m.runActionCmd()
			}
		case "d":
			if m.selected != nil {
				m.action = "delete"
				m.state = stateRunning
				return m, m.runActionCmd()
			}
		case "esc", "q":
			m.state = stateList
			m.selected = nil
			return m, nil
		}
	case stateRunning:
		// Ignorar keys.
	case stateDone:
		// Cualquier key vuelve a la lista.
		m.state = stateList
		m.err = nil
		m.selected = nil
		m.action = ""
		return m, nil
	}
	return m, nil
}

func (m *Model) View() string {
	switch m.state {
	case stateList:
		return m.viewList()
	case stateAction:
		return m.viewAction()
	case stateRunning:
		return "\n  Running action...\n"
	case stateDone:
		return m.viewDone()
	}
	return ""
}

func (m *Model) viewList() string {
	s := "\n"
	s += styles.Title.Render("  Backups") + "\n"
	if m.err != nil {
		s += styles.Fail.Render("  Error: ") + m.err.Error() + "\n"
	}
	if len(m.backups) == 0 {
		s += styles.ItemDesc.Render("  (no backups found)") + "\n"
	} else {
		for i, b := range m.backups {
			cursor := "   "
			st := styles.ItemTitle
			if i == m.cursor {
				cursor = "  > "
				st = styles.ItemSelected
			}
			s += fmt.Sprintf("%s%s\n", cursor, st.Render(b.Path))
		}
	}
	s += "\n"
	s += styles.HelpText.Render("  [up/down] move   [enter] select   [esc] back") + "\n"
	return s
}

func (m *Model) viewAction() string {
	if m.selected == nil {
		return ""
	}
	s := "\n"
	s += fmt.Sprintf("  Selected: %s\n\n", styles.ItemTitle.Render(m.selected.Path))
	s += "  Action:\n"
	s += "    [r] restore   replace original with this backup\n"
	s += "    [d] delete    remove this backup file\n"
	s += "    [esc] cancel\n"
	return s
}

func (m *Model) viewDone() string {
	s := "\n"
	if m.err != nil {
		s += styles.Fail.Render("  Failed: ") + m.err.Error() + "\n"
	} else {
		s += styles.Ok.Render(fmt.Sprintf("  %s done.", m.action)) + "\n"
	}
	s += "\n  Press any key to return to list.\n"
	return s
}

func (m *Model) runActionCmd() tea.Cmd {
	sel := m.selected
	action := m.action
	runner := m.runner
	return func() tea.Msg {
		if sel == nil {
			return runResultMsg{err: fmt.Errorf("no selection")}
		}
		if action == "delete" {
			err := os.Remove(sel.Path)
			return runResultMsg{err: err}
		}
		// restore: invoca 'domain restore <path>'
		err := runner(context.Background(), []string{"restore", sel.Path})
		return runResultMsg{err: err}
	}
}

func backCmd() tea.Cmd {
	return func() tea.Msg { return menu.BackMsg{} }
}

// listBackups lista todos los backups en paths canonicos.
func listBackups() ([]backupEntry, error) {
	paths := []string{
		install.CredentialsPath(),
		".env",
		openCodeConfigPath(),
	}
	var entries []backupEntry
	for _, p := range paths {
		backups, err := install.ListBackups(p)
		if err != nil {
			return nil, err
		}
		for _, b := range backups {
			info, statErr := os.Stat(b)
			size := int64(0)
			if statErr == nil {
				size = info.Size()
			}
			entries = append(entries, backupEntry{
				Path:     b,
				Original: p,
				Size:     size,
			})
		}
	}
	return entries, nil
}

func openCodeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

func defaultRestoreRunner(ctx context.Context, args []string) error {
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
