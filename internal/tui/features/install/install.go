// Package install — TUI feature para `domain install` (HU-01.11).
//
// Flow dentro de la TUI:
//   1. Pantalla de bienvenida + chequeo de deps (Go/git, [docker si local])
//      - Si falta algo critico, ofrece auto-install con confirm.
//   2. Wizard de 4 prompts: mode / base-url / init y-n / opencode y-n
//   3. Run install con InstallProgress (5 steps + summary)
//   4. Vuelve al menu con BackMsg
//
// La TUI solo orquesta; la logica real vive en internal/installer
// y cmd/domain/install_cli.go (reusado via runInstallFromFlags).

package install

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"nunezlagos/domain/internal/installer"
	"nunezlagos/domain/internal/tui/menu"
	"nunezlagos/domain/internal/tui/styles"
)

type state int

const (
	stateWelcome state = iota
	stateDepCheck
	stateModePrompt
	stateBaseURLPrompt
	stateInitPrompt
	stateOpencodePrompt
	stateRunning
	stateDone
)

// Mode seleccionado en el prompt.
type modeSel int

const (
	modeLocal modeSel = iota
	modeCloud
	modeHybrid
)

func (m modeSel) String() string {
	switch m {
	case modeLocal:
		return "local"
	case modeCloud:
		return "cloud"
	case modeHybrid:
		return "hybrid"
	}
	return "?"
}

// Model bubbletea para la feature install.
type Model struct {
	state    state
	platform installer.Platform
	deps     []installer.CheckResult
	mode     modeSel
	baseURL  string
	doInit   bool
	doOpencode bool
	confirm  installer.ConfirmFunc
	err      error
}

// New crea un Model con confirm default (prompt via stdin/stdout).
func New() *Model {
	return &Model{
		state:    stateWelcome,
		baseURL:  "http://localhost:8000",
		doInit:   false,
		doOpencode: true,
		confirm:  defaultConfirm,
	}
}

// Init implementa tea.Model. Dispara la deteccion de platform.
func (m *Model) Init() tea.Cmd {
	return m.detectPlatformCmd()
}

func (m *Model) detectPlatformCmd() tea.Cmd {
	return func() tea.Msg {
		p, err := installer.DetectPlatform()
		if err != nil {
			return platformMsg{err: err}
		}
		return platformMsg{platform: p}
	}
}

type platformMsg struct {
	platform installer.Platform
	err      error
}

// Update implementa tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case platformMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateDone
			return m, nil
		}
		m.platform = msg.platform
		m.state = stateModePrompt
		return m, nil
	case depsMsg:
		m.deps = msg.deps
		m.state = stateBaseURLPrompt
		return m, nil
	case runResultMsg:
		if msg.err != nil {
			m.err = msg.err
		}
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
	case stateWelcome:
		if key == "enter" || key == " " {
			m.state = stateModePrompt
			return m, nil
		}
		if key == "esc" || key == "q" {
			return m, backCmd()
		}
	case stateDepCheck:
		// Legacy state (no se usa mas). Mantenido por compat.
		// Auto-avanza al prompt de mode; sin interaccion.
	case stateModePrompt:
		switch key {
		case "1":
			m.mode = modeLocal
			m.state = stateDepCheck
			return m, m.checkDepsCmd()
		case "2":
			m.mode = modeCloud
			m.state = stateDepCheck
			return m, m.checkDepsCmd()
		case "3":
			m.mode = modeHybrid
			// Hybrid no esta implementado end-to-end: caemos a local con warning.
			m.mode = modeLocal
			m.state = stateDepCheck
			return m, m.checkDepsCmd()
		}
	case stateBaseURLPrompt:
		// Manejado por el input prompt (no por keymap).
		// Si el user presiona enter sin texto, usa default.
		if key == "enter" {
			if m.baseURL == "" {
				m.baseURL = "http://localhost:8000"
			}
			m.state = stateInitPrompt
			return m, nil
		}
		if key == "backspace" {
			if len(m.baseURL) > 0 {
				m.baseURL = m.baseURL[:len(m.baseURL)-1]
			}
			return m, nil
		}
		// Append printable char.
		if len(key) == 1 {
			m.baseURL += key
			return m, nil
		}
	case stateInitPrompt:
		switch key {
		case "y", "Y":
			m.doInit = true
			m.state = stateOpencodePrompt
			return m, nil
		case "n", "N":
			m.doInit = false
			m.state = stateOpencodePrompt
			return m, nil
		case "enter":
			// Default: n
			m.doInit = false
			m.state = stateOpencodePrompt
			return m, nil
		}
	case stateOpencodePrompt:
		switch key {
		case "y", "Y":
			m.doOpencode = true
			m.state = stateRunning
			return m, m.runInstallCmd()
		case "n", "N":
			m.doOpencode = false
			m.state = stateRunning
			return m, m.runInstallCmd()
		case "enter":
			// Default: y
			m.doOpencode = true
			m.state = stateRunning
			return m, m.runInstallCmd()
		}
	case stateDone:
		// Cualquier key vuelve al menu.
		return m, backCmd()
	}
	return m, nil
}

// View implementa tea.Model.
func (m *Model) View() string {
	switch m.state {
	case stateWelcome:
		return m.viewWelcome()
	case stateDepCheck:
		return m.viewDepCheck()
	case stateModePrompt:
		return m.viewModePrompt()
	case stateBaseURLPrompt:
		return m.viewBaseURLPrompt()
	case stateInitPrompt:
		return m.viewInitPrompt()
	case stateOpencodePrompt:
		return m.viewOpencodePrompt()
	case stateRunning:
		return m.viewRunning()
	case stateDone:
		return m.viewDone()
	}
	return ""
}

func (m *Model) viewWelcome() string {
	s := "\n"
	s += styles.Title.Render("  Domain Install") + "\n"
	s += styles.ItemDesc.Render("  Press enter to start, esc to go back") + "\n"
	s += "\n"
	s += fmt.Sprintf("  Detected: %s/%s (%s)\n", m.platform.OS, m.platform.Distro, m.platform.PkgMgr)
	return s
}

func (m *Model) viewDepCheck() string {
	s := "\n  Checking dependencies...\n\n"
	for _, r := range m.deps {
		status := styles.Fail.Render("[ MISSING ]")
		if r.Found {
			if r.Dep.MinVer != "" && !r.MinMet {
				status = styles.Warn.Render(fmt.Sprintf("[ %s < %s ]", r.Version, r.Dep.MinVer))
			} else {
				status = styles.Ok.Render("[ ok ]")
			}
		}
		s += fmt.Sprintf("  %s  %s (%s)\n", status, r.Dep.Name, r.Dep.Binary)
		if !r.Found && r.Hint != "" {
			s += fmt.Sprintf("           %s\n", styles.ItemDesc.Render(r.Hint))
		}
	}
	s += "\n"
	return s
}

func (m *Model) viewModePrompt() string {
	s := "\n  Deployment mode:\n"
	s += "    1) local   — docker compose (Postgres+S3+SMTP)\n"
	s += "    2) cloud   — bring your own services (DSN)\n"
	s += "    3) hybrid  — mix per-service (falls back to local)\n"
	s += "  Choice [1]: "
	return s
}

func (m *Model) viewBaseURLPrompt() string {
	s := "\n  Domain server URL\n"
	s += fmt.Sprintf("  [%s]: ", m.baseURL)
	return s
}

func (m *Model) viewInitPrompt() string {
	s := "\n  Archive .md files to BD (init)?\n"
	s += "  [y/N]: "
	return s
}

func (m *Model) viewOpencodePrompt() string {
	s := "\n  Configure opencode MCP server?\n"
	s += "  [Y/n]: "
	return s
}

func (m *Model) viewRunning() string {
	return "\n  Running install (see install_cli.go output)...\n"
}

func (m *Model) viewDone() string {
	s := "\n"
	if m.err != nil {
		s += styles.Fail.Render("  Install failed: ") + m.err.Error() + "\n"
	} else {
		s += styles.Ok.Render("  Install complete.") + "\n"
	}
	s += "\n  Press any key to return to menu.\n"
	return s
}

// --- Comandos async ---

type depsMsg struct {
	deps []installer.CheckResult
}

func (m *Model) checkDepsCmd() tea.Cmd {
	deps := depsForMode(m.mode)
	return func() tea.Msg {
		results := installer.Check(deps)
		return depsMsg{deps: results}
	}
}

// depsForMode retorna las deps a chequear segun el deployment mode.
// - local: go, git, docker (docker lo necesita para compose)
// - cloud: go, git (cloud trae su propio Postgres)
// - hybrid: go, git, docker (hybrid suele incluir local en algun servicio)
func depsForMode(m modeSel) []installer.Dep {
	base := []installer.Dep{installer.DepGo, installer.DepGit}
	switch m {
	case modeLocal, modeHybrid:
		base = append(base, installer.DepDocker)
	}
	return base
}

type runResultMsg struct {
	err error
}

func (m *Model) runInstallCmd() tea.Cmd {
	mode := m.mode.String()
	flags := []string{
		"--mode", mode,
		"--base-url", m.baseURL,
		"--non-interactive",
	}
	if !m.doInit {
		flags = append(flags, "--no-init")
	}
	if !m.doOpencode {
		flags = append(flags, "--no-opencode")
	}
	// Aqui llamamos a la logica real. Para mantener la TUI testable
	// sin ejecutar el install (que toca DB, docker, etc.), exportamos
	// una funcion RunInstallWithFlags(flags) que retorna error.
	// Ver install_runner.go en este paquete.
	return func() tea.Msg {
		err := runInstallWithFlags(context.Background(), flags)
		return runResultMsg{err: err}
	}
}

// --- helpers ---

func backCmd() tea.Cmd {
	return func() tea.Msg { return menu.BackMsg{} }
}

func defaultConfirm(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	var resp string
	fmt.Scanln(&resp)
	resp = strings.ToLower(strings.TrimSpace(resp))
	return resp == "" || resp == "y" || resp == "yes"
}
