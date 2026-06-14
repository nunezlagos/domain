// Package app — root state machine del TUI de domain (HU-01.11).
//
// State machine: stateMenu ↔ stateFeature.
//   stateMenu: muestra el menu principal.
//   stateFeature: una feature activa (install/update/backups).
//                 Cuando termina, envia BackMsg{} → vuelve al menu.
//   stateExit: tea.Quit.
//
// Patron copiado de ptools/internal/app/app.go.

package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"nunezlagos/domain/internal/tui/features/backups"
	"nunezlagos/domain/internal/tui/features/install"
	"nunezlagos/domain/internal/tui/features/update"
	"nunezlagos/domain/internal/tui/menu"
)

type state int

const (
	stateMenu state = iota
	stateFeature
)

// BackMsg es un alias para menu.BackMsg (mantiene compatibilidad
// con callers que ya lo importan desde app).
type BackMsg = menu.BackMsg

// ExitMsg enviado cuando el user elige Exit.
type ExitMsg struct{}

// Model es el root bubbletea model.
type Model struct {
	state   state
	menu    menu.Model
	current tea.Model
}

// New arranca en el menu.
func New() Model {
	return Model{state: stateMenu, menu: menu.New()}
}

// NewDirect arranca en una feature (e.g., `domain install` CLI → feature install).
func NewDirect(featureIdx int) Model {
	m := Model{state: stateFeature, menu: menu.New(), current: featureFor(featureIdx)}
	return m
}

// Init implementa tea.Model.
func (m Model) Init() tea.Cmd {
	if m.state == stateFeature && m.current != nil {
		return m.current.Init()
	}
	return m.menu.Init()
}

// Update implementa tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateMenu:
		return m.updateMenu(msg)
	case stateFeature:
		return m.updateFeature(msg)
	}
	return m, nil
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case menu.SelectMsg:
		if msg.Index == menu.IndexExit {
			return m, tea.Quit
		}
		feat := featureFor(msg.Index)
		if feat == nil {
			// Unknown feature: stay in menu
			return m, nil
		}
		m.state = stateFeature
		m.current = feat
		return m, feat.Init()
	}
	// Delegate key updates al menu
	var cmd tea.Cmd
	updated, cmd := m.menu.Update(msg)
	if concrete, ok := updated.(menu.Model); ok {
		m.menu = concrete
	}
	return m, cmd
}

func (m Model) updateFeature(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Si la feature envia BackMsg, volvemos al menu.
	if _, ok := msg.(BackMsg); ok {
		m.state = stateMenu
		m.current = nil
		return m, nil
	}
	// ctrl+c SIEMPRE sale, esté donde esté la feature. Sin esto el user
	// queda atrapado en features que no manejan la key.
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.current == nil {
		return m, nil
	}
	var cmd tea.Cmd
	m.current, cmd = m.current.Update(msg)
	return m, cmd
}

// View implementa tea.Model.
func (m Model) View() string {
	if m.state == stateFeature && m.current != nil {
		return m.current.View()
	}
	return m.menu.View()
}

// featureFor mapea index del menu → tea.Model de la feature.
// Retorna nil si el index no corresponde a una feature.
func featureFor(index int) tea.Model {
	switch index {
	case menu.IndexInstall:
		return install.New()
	case menu.IndexUpdate:
		return update.New()
	case menu.IndexBackups:
		return backups.New()
	}
	return nil
}
