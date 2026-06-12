// Package styles — lipgloss styles compartidos por el TUI de domain
// (HU-01.11). Paleta alineada con ptools (personal-tools). Mantener
// este archivo chico: solo constantes y estilos globales.

package styles

import "github.com/charmbracelet/lipgloss"

// Paleta (espejo de personal-tools/internal/tui/styles).
var (
	Primary   = lipgloss.Color("#9C4C7A") // púrpura ptools (selected bg, accents)
	Secondary = lipgloss.Color("#5A9E6F") // verde success
	Muted     = lipgloss.Color("#888888") // gris medio (descripciones)
	Selected  = lipgloss.Color("#FFFFFF") // blanco (fg del item seleccionado)
	Danger    = lipgloss.Color("#CC4444") // rojo suave
	Help      = lipgloss.Color("#555555") // gris oscuro (help bar)
	TitleFg   = lipgloss.Color("#F0F0F0") // blanco grisáceo (títulos)
	WarnColor = lipgloss.Color("#D7AF5F") // ámbar
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
	Foreground(lipgloss.Color("#999999")).
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
