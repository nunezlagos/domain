// Package update — TUI feature para `domain update` (HU-01.11).
// Stub en commit 2/5; completo en commit 4/5.

package update

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct{}

func New() *Model { return &Model{} }

func (m *Model) Init() tea.Cmd { return nil }
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return m, func() tea.Msg { return struct{}{} }
	}
	return m, nil
}
func (m *Model) View() string {
	return "\n  [update feature - placeholder, complete in commit 4/5]\n\n  press any key to go back\n"
}
