





package menu

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"nunezlagos/domain/internal/tui/styles"
)

// Indexes de los items del menu (estables, no reordenar).
const (
	IndexInstall = 0
	IndexUpdate  = 1
	IndexBackups = 2
	IndexExit    = 3
)

// SelectMsg se envia cuando el user selecciona un item.
type SelectMsg struct {
	Index int
}

// BackMsg enviado por features para volver al menu.
// Vive aca (en menu) para evitar import cycle: features lo importan
// y app tambien (app.go importa menu.go).
type BackMsg struct{}

// item es un item del menu (interno).
type item struct {
	title string
	desc  string
}

// defaultItems lista canonica. El Index constante arriba debe matchear.
var defaultItems = []item{
	{"Install", "deploy mode + migrate + seed + agent setup"},
	{"Update", "backups + migrate + seed (idempotent)"},
	{"Backups", "list, restore, or delete backup files"},
	{"Exit", "quit domain TUI"},
}

// FeatureNames retorna nombres lowercase (para help).
func FeatureNames() []string {
	names := make([]string, len(defaultItems))
	for i, it := range defaultItems {
		names[i] = it.title
	}
	return names
}

// IndexOf retorna el index del item con ese title (case-insensitive).
// Usado por el CLI dispatch (e.g., "domain install" → IndexInstall).
func IndexOf(name string) int {
	lower := toLower(name)
	for i, it := range defaultItems {
		if toLower(it.title) == lower {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

// Model es el bubbletea Model del menu principal.
type Model struct {
	items  []item
	cursor int
}

// New crea un Model del menu.
func New() Model {
	return Model{items: defaultItems, cursor: 0}
}

// Init implementa tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implementa tea.Model (interface). Retorna tea.Model para
// cumplir la interfaz (no Model concreto).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			idx := m.cursor
			return m, func() tea.Msg { return SelectMsg{Index: idx} }
		case "1", "2", "3", "4":
			n := int(msg.String()[0] - '0')
			if n >= 1 && n <= len(m.items) {
				m.cursor = n - 1
				idx := m.cursor
				return m, func() tea.Msg { return SelectMsg{Index: idx} }
			}
		case "q", "ctrl+c", "esc":
			return m, func() tea.Msg { return SelectMsg{Index: IndexExit} }
		}
	}
	return m, nil
}

// View implementa tea.Model.
func (m Model) View() string {
	s := "\n"
	s += styles.Title.Render("  DOMAIN") + "  " + styles.Subtitle.Render("v0.3.0-installer") + "\n"
	s += styles.ItemDesc.Render("  personal & project memory platform") + "\n\n"

	for i, it := range m.items {
		num := fmt.Sprintf("  %d.", i+1)
		var titleSt, descSt = styles.ItemTitle, styles.ItemDesc

		cursor := "   "
		if i == m.cursor {
			cursor = "  > "
			titleSt = styles.ItemSelected
		}

		row := cursor + num + " " + titleSt.Render(it.title)
		if it.desc != "" {
			row += "  " + descSt.Render(it.desc)
		}
		s += row + "\n"
	}

	s += "\n"
	s += styles.HelpText.Render("  [enter/1-4] select   [up/down] move   [q] quit") + "\n"
	return s
}
