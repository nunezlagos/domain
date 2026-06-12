// Selectable list component (HU-01.13, rediseño 2026-06-11).
//
// Semántica nueva: enter/espacio sobre un item lo SELECCIONA (o togglea
// en modo multi) pero NO avanza. Para avanzar, el user baja al botón
// [ Continuar ] y presiona enter ahí. Esto evita avances accidentales
// con enter en todas las vistas del wizard.
//
// Diseño:
//   - Cursor: >  (a la izquierda del item)
//   - Single: (•) elegido / ( ) no elegido
//   - Multi:  [X] marcado / [ ] desmarcado
//   - Disabled: [-] (gris, no elegible)
//   - Footer: botón [ Continuar ] (último slot navegable)
//
// Mensajes emitidos:
//   - SelectMsg{Index}        al confirmar en Continuar (modo single)
//   - MultiSelectMsg{Indices} al confirmar en Continuar (modo multi)
//   - CancelMsg               en esc/q/ctrl+c

package selectable

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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
	cursor   int // 0..len(items)-1 = items; len(items) = botón Continuar
	selected int // single: índice elegido
	checked  []bool
	multi    bool
	title    string
	helpText string
}

// New crea un selectable single-choice. El primer item habilitado queda
// pre-seleccionado para que Continuar siempre sea válido.
func New(title string, items []Item) Model {
	sel := -1
	for i, it := range items {
		if !it.Disabled {
			sel = i
			break
		}
	}
	return Model{
		items:    items,
		selected: sel,
		checked:  make([]bool, len(items)),
		title:    title,
		helpText: "[↑/↓] mover   [espacio/enter] elegir   [enter] en Continuar avanza   [esc] volver",
	}
}

// NewMulti crea un selectable multi-choice. defaults son los índices
// pre-marcados.
func NewMulti(title string, items []Item, defaults []int) Model {
	m := New(title, items)
	m.multi = true
	m.selected = -1
	for _, idx := range defaults {
		if idx >= 0 && idx < len(items) && !items[idx].Disabled {
			m.checked[idx] = true
		}
	}
	m.helpText = "[↑/↓] mover   [espacio/enter] marcar/desmarcar   [enter] en Continuar avanza   [esc] volver"
	return m
}

// continueSlot es el índice del botón Continuar en el espacio de cursor.
func (m Model) continueSlot() int { return len(m.items) }

// Init implementa tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implementa tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	key := keyMsg.String()

	// Number keys: jump + elegir/togglear el item (sin avanzar).
	if len(key) == 1 && key >= "1" && key <= "9" {
		n := int(key[0]-'0') - 1
		if n >= 0 && n < len(m.items) && !m.items[n].Disabled {
			m.cursor = n
			m.pick(n)
		}
		return m, nil
	}

	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		} else {
			m.cursor = m.continueSlot() // wrap al botón
		}
	case "down", "j":
		if m.cursor < m.continueSlot() {
			m.cursor++
		} else {
			m.cursor = 0 // wrap
		}
	case "tab":
		// Atajo directo al botón Continuar.
		m.cursor = m.continueSlot()
	case " ", "space":
		if m.cursor < len(m.items) {
			m.pick(m.cursor)
		}
	case "enter":
		if m.cursor == m.continueSlot() {
			return m, m.confirmCmd()
		}
		m.pick(m.cursor)
	case "q", "esc", "ctrl+c":
		return m, func() tea.Msg { return CancelMsg{} }
	}
	return m, nil
}

// pick selecciona (single) o togglea (multi) el item idx.
func (m *Model) pick(idx int) {
	if idx < 0 || idx >= len(m.items) || m.items[idx].Disabled {
		return
	}
	if m.multi {
		m.checked[idx] = !m.checked[idx]
		return
	}
	m.selected = idx
}

// confirmCmd emite el mensaje de confirmación según el modo.
func (m Model) confirmCmd() tea.Cmd {
	if m.multi {
		var indices []int
		for i, c := range m.checked {
			if c {
				indices = append(indices, i)
			}
		}
		return func() tea.Msg { return MultiSelectMsg{Indices: indices} }
	}
	if m.selected < 0 {
		return nil // nada elegido: Continuar no hace nada
	}
	idx := m.selected
	return func() tea.Msg { return SelectMsg{Index: idx} }
}

// View implementa tea.Model.
func (m Model) View() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(styles.Title.Render(m.title))
	sb.WriteString("\n\n")

	for i, item := range m.items {
		cursor := "    "
		if i == m.cursor {
			cursor = "  > "
		}

		check := m.checkbox(i, item)

		label := fmt.Sprintf("%s%s %s", cursor, check, item.Label)
		if i == m.cursor {
			label = fmt.Sprintf("%s%s %s", cursor, check, styles.ItemSelected.Render(" "+item.Label+" "))
		} else if item.Disabled {
			label = styles.ItemDesc.Render(label)
		}
		sb.WriteString(label)
		sb.WriteString("\n")

		if item.Description != "" {
			sb.WriteString(styles.ItemDesc.Render("        " + item.Description))
			sb.WriteString("\n")
		}
	}

	// Botón Continuar
	sb.WriteString("\n")
	btn := styles.Button.Render("[ Continuar ]")
	prefix := "    "
	if m.cursor == m.continueSlot() {
		btn = styles.ButtonFocused.Render("[ Continuar ]")
		prefix = "  > "
	}
	sb.WriteString(prefix + btn + "\n")

	sb.WriteString("\n")
	sb.WriteString("  ")
	sb.WriteString(styles.HelpText.Render(m.helpText))
	sb.WriteString("\n")
	return sb.String()
}

// checkbox renderiza el indicador del item según modo y estado.
func (m Model) checkbox(i int, item Item) string {
	muted := styles.ItemDesc
	switch {
	case item.Disabled:
		return muted.Render("[-]")
	case m.multi && m.checked[i]:
		return styles.Ok.Render("[X]")
	case m.multi:
		return muted.Render("[ ]")
	case i == m.selected:
		return styles.Ok.Render("(•)")
	default:
		return muted.Render("( )")
	}
}

// SelectedIndex retorna el index seleccionado (-1 si no). Solo single.
func (m Model) SelectedIndex() int { return m.selected }

// CheckedIndices retorna los índices marcados (modo multi).
func (m Model) CheckedIndices() []int {
	var out []int
	for i, c := range m.checked {
		if c {
			out = append(out, i)
		}
	}
	return out
}

// Items retorna los items (defensive copy).
func (m Model) Items() []Item {
	out := make([]Item, len(m.items))
	copy(out, m.items)
	return out
}

// SelectMsg emitido al confirmar en Continuar (modo single).
type SelectMsg struct {
	Index int
}

// MultiSelectMsg emitido al confirmar en Continuar (modo multi).
type MultiSelectMsg struct {
	Indices []int
}

// CancelMsg emitido cuando el user cancela (esc/q/ctrl+c).
type CancelMsg struct{}
