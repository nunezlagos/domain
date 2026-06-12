// Selectable list component (HU-01.13 commit 2/3).
//
// Reemplaza los prompts [1/2/3] y [y/n] de HU-01.11 con widgets
// visuales tipo [X] (seleccionado) / [ ] (no seleccionado).
// El user navega con ↑/↓ y confirma con Enter.
//
// Diseño:
//   - Cursor: >  (a la izquierda del item)
//   - Item seleccionado: [X] ...texto
//   - Item no seleccionado: [ ] ...texto
//   - Items disabled: [-] ...texto (gris, no se puede elegir)
//
// El callback onSelect se invoca con el index del item elegido.

package selectable

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"nunezlagos/domain/internal/tui/styles"
)

// Item representa una opcion en el selectable list.
type Item struct {
	Label       string
	Description string
	Disabled    bool
}

// Model es el bubbletea Model del selectable list.
type Model struct {
	items    []Item
	cursor   int
	selected int            // -1 si no se ha elegido todavia
	quitting bool
	title    string
	helpText string
}

// New crea un Model con los items dados y un titulo.
func New(title string, items []Item) Model {
	return Model{
		items:    items,
		cursor:   0,
		selected: -1,
		title:    title,
		helpText: "[↑/↓] move   [enter] select   [esc] cancel",
	}
}

// Init implementa tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implementa tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		// Number keys: jump directo al item
		if len(key) == 1 && key >= "1" && key <= "9" {
			n := int(key[0] - '0')
			if n >= 1 && n <= len(m.items) {
				idx := n - 1
				if !m.items[idx].Disabled {
					m.cursor = idx
					m.selected = idx
					return m, func() tea.Msg { return SelectMsg{Index: idx} }
				}
				// Disabled: no jump, no select
				return m, nil
			}
		}
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.items) - 1 // wrap
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			} else {
				m.cursor = 0 // wrap
			}
		case "enter":
			// Skip si el item es disabled
			if m.items[m.cursor].Disabled {
				return m, nil
			}
			m.selected = m.cursor
			return m, func() tea.Msg { return SelectMsg{Index: m.cursor} }
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, func() tea.Msg { return CancelMsg{} }
		}
	}
	return m, nil
}

// View implementa tea.Model.
func (m Model) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(styles.Title.Render(m.title))
	sb.WriteString("\n\n")

	for i, item := range m.items {
		cursor := "   "
		var lineStyle lipgloss.Style

		if i == m.cursor {
			cursor = "  > "
			lineStyle = styles.ItemSelected
		} else {
			lineStyle = styles.ItemTitle
		}

		// Checkbox icon
		var check string
		muted := lipgloss.NewStyle().Foreground(styles.Muted)
		switch {
		case item.Disabled:
			check = muted.Render("[-]")
		case i == m.selected:
			check = styles.Ok.Render("[X]")
		default:
			check = muted.Render("[ ]")
		}

		label := lineStyle.Render(fmt.Sprintf("%s %s %s", cursor, check, item.Label))
		sb.WriteString(label)
		sb.WriteString("\n")

		// Descripcion
		if item.Description != "" {
			desc := styles.ItemDesc.Render("      " + item.Description)
			sb.WriteString(desc)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(styles.HelpText.Render(m.helpText))
	sb.WriteString("\n")
	return sb.String()
}

// SelectedIndex retorna el index seleccionado (-1 si no).
func (m Model) SelectedIndex() int {
	return m.selected
}

// Items retorna los items (defensive copy).
func (m Model) Items() []Item {
	out := make([]Item, len(m.items))
	copy(out, m.items)
	return out
}

// SelectMsg emitido cuando el user elige un item.
type SelectMsg struct {
	Index int
}

// CancelMsg emitido cuando el user cancela (esc/q/ctrl+c).
type CancelMsg struct{}
