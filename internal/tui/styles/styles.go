// Package styles — lipgloss styles compartidos por el TUI de domain
// (HU-01.11). Paleta alineada con ptools (personal-tools). Mantener
// este archivo chico: solo constantes y estilos globales.

package styles

import "github.com/charmbracelet/lipgloss"

// Paleta (espejo de personal-tools, ajustada para terminal con fondo
// negro: ningún gris por debajo de ~#8A para que el texto siempre se lea).
var (
	Primary   = lipgloss.Color("#B05A8E") // púrpura ptools (un punto más claro para fondo negro)
	Secondary = lipgloss.Color("#6FBF85") // verde success legible en negro
	Muted     = lipgloss.Color("#9E9E9E") // gris descripciones (visible en negro)
	Selected  = lipgloss.Color("#FFFFFF") // blanco (fg del item seleccionado)
	Danger    = lipgloss.Color("#E06C6C") // rojo legible en negro
	Help      = lipgloss.Color("#8A8A8A") // help bar — antes #555, invisible en negro
	TitleFg   = lipgloss.Color("#F0F0F0") // blanco grisáceo (títulos)
	WarnColor = lipgloss.Color("#E5C07B") // ámbar
)

// Title estilo para headers.
var Title = lipgloss.NewStyle().
	Bold(true).
	Foreground(TitleFg).
	Padding(0, 1)

// Subtitle estilo para el subtitulo (version, etc).
var Subtitle = lipgloss.NewStyle().
	Foreground(Muted).
	Italic(true)

// ItemTitle estilo para items no-seleccionados.
var ItemTitle = lipgloss.NewStyle().
	Foreground(TitleFg)

// ItemSelected estilo para el item bajo el cursor.
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

// Prompt estilo para prompts de input.
var Prompt = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#B0B0B0")).
	Italic(true)

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
	Foreground(WarnColor)

// Accent estilo para valores destacados (config elegida, paths).
var Accent = lipgloss.NewStyle().
	Foreground(Primary).
	Bold(true)

// Button estilo para el botón Continuar sin foco.
var Button = lipgloss.NewStyle().
	Foreground(TitleFg).
	Padding(0, 2)

// ButtonFocused estilo para el botón Continuar con foco.
var ButtonFocused = lipgloss.NewStyle().
	Bold(true).
	Foreground(Selected).
	Background(Primary).
	Padding(0, 2)
