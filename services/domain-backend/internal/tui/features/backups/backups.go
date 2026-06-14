// Package backups — stub mínimo del feature backups del TUI (HU-01.11).
// Implementa tea.Model con vista placeholder hasta que se desarrolle la
// funcionalidad real. Permite que `internal/tui/app/app.go` compile.
package backups

import tea "github.com/charmbracelet/bubbletea"

type Model struct{}

func New() *Model { return &Model{} }

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) View() string {
	return "\n  Backups TUI: pendiente de implementar.\n\n  ESC/q para volver al menú.\n"
}
