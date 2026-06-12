// Package styles — lipgloss styles compartidos por el TUI de domain
// (HU-01.11). Mantener este archivo chico: solo constantes y
// estilos globales. Si crece, partirlo en archivos por dominio.

package styles

import "github.com/charmbracelet/lipgloss"

// Paleta (consistente con ptools, simplificada).
var (
	Primary   = lipgloss.Color("#7D56F4") // purple-ish
	Secondary = lipgloss.Color("#04B575") // green-ish
	Muted     = lipgloss.Color("#626262") // gray
	Selected  = lipgloss.Color("#F0F0F0")
	Danger    = lipgloss.Color("#FF5F87")
	Help      = lipgloss.Color("#8F8F8F")
)

// Title estilo para el header del menu.
var Title = lipgloss.NewStyle().
	Bold(true).
	Foreground(Primary).
	Padding(0, 1)

// Subtitle estilo para el subtitulo (version, etc).
var Subtitle = lipgloss.NewStyle().
	Foreground(Muted).
	Italic(true)

// ItemTitle estilo para items del menu no-seleccionados.
var ItemTitle = lipgloss.NewStyle().
	Foreground(Secondary)

// ItemSelected estilo para el item seleccionado.
var ItemSelected = lipgloss.NewStyle().
	Bold(true).
	Foreground(Selected).
	Background(Primary)

// ItemDesc estilo para descripcion del item.
var ItemDesc = lipgloss.NewStyle().
	Foreground(Muted)

// HelpKey estilo para las keys del help bar.
var HelpKey = lipgloss.NewStyle().
	Foreground(Primary).
	Bold(true)

// HelpText estilo para el help bar completo.
var HelpText = lipgloss.NewStyle().
	Foreground(Help)

// Ok estilo para resultados exitosos.
var Ok = lipgloss.NewStyle().
	Foreground(Secondary).
	Bold(true)

// Fail estilo para resultados fallidos.
var Fail = lipgloss.NewStyle().
	Foreground(Danger).
	Bold(true)

// Warn estilo para warnings.
var Warn = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFAF00"))
