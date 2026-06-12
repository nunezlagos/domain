// Package install — TUI feature para `domain install` (HU-01.11 + 01.13).
//
// Flow con widgets visuales (HU-01.13):
//   1. welcome
//   2. modePrompt: SELECTABLE [X] local / [ ] cloud / [-] hybrid
//   3. depCheck: go/git/[docker] (segun mode)
//   4. baseURLPrompt: textinput
//   5. initPrompt: SELECTABLE [X] yes / [ ] no
//   6. opencodePrompt: SELECTABLE [X] yes / [ ] no
//   7. running: install via sub-process con error propagation
//   8. done
//
// Cualquier key en done → BackMsg → vuelve al menu.

package install

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"nunezlagos/domain/internal/installer"
	"nunezlagos/domain/internal/tui/menu"
	"nunezlagos/domain/internal/tui/selectable"
	"nunezlagos/domain/internal/tui/styles"
)

type state int

const (
	stateWelcome state = iota
	stateModePrompt
	stateDepCheck
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

// Sub-models que la feature integra.
type subModel interface {
	Update(tea.Msg) (tea.Model, tea.Cmd)
	View() string
	Init() tea.Cmd
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
	// stderr del sub-process (para error propagation)
	stderr string
	// sub-models de los prompts
	modePrompt   selectable.Model
	initPrompt   selectable.Model
	opencodePrompt selectable.Model
}

func New() *Model {
	return &Model{
		state:   stateWelcome,
		baseURL: "http://localhost:8000",
		doOpencode: true,
		confirm: defaultConfirm,
		modePrompt: selectable.New("Deployment mode", []selectable.Item{
			{Label: "local", Description: "docker compose (Postgres + S3 + SMTP)"},
			{Label: "cloud", Description: "bring your own services (DSN)"},
			{Label: "hybrid", Description: "mix per-service (falls back to local)", Disabled: true},
		}),
		initPrompt: selectable.New("Archive .md files to BD (init)?", []selectable.Item{
			{Label: "yes", Description: "Backup CLAUDE.md, .claude/**, .opencode/** to BD"},
			{Label: "no", Description: "Skip init step (faster install)"},
		}),
		opencodePrompt: selectable.New("Configure opencode MCP server?", []selectable.Item{
			{Label: "yes", Description: "Run 'domain setup opencode' after install"},
			{Label: "no", Description: "Skip agent setup (configure manually later)"},
		}),
	}
}

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

type depsMsg struct {
	deps []installer.CheckResult
}

type runResultMsg struct {
	err    error
	stderr string
}

// Update implementa tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Si estamos en un sub-prompt, delegamos al sub-model primero.
	switch m.state {
	case stateModePrompt:
		// Detectar SelectMsg del sub-prompt
		if sel, ok := msg.(selectable.SelectMsg); ok {
			m.setMode(sel.Index)
			m.state = stateDepCheck
			return m, m.checkDepsCmd()
		}
		if _, ok := msg.(selectable.CancelMsg); ok {
			return m, backCmd()
		}
		// Delegar el resto de keys al sub-model
		updated, cmd := m.modePrompt.Update(msg)
		m.modePrompt = updated.(selectable.Model)
		return m, cmd
	case stateInitPrompt:
		if sel, ok := msg.(selectable.SelectMsg); ok {
			m.doInit = sel.Index == 0
			m.state = stateOpencodePrompt
			return m, nil
		}
		if _, ok := msg.(selectable.CancelMsg); ok {
			m.state = stateModePrompt // back to mode
			return m, nil
		}
		updated, cmd := m.initPrompt.Update(msg)
		m.initPrompt = updated.(selectable.Model)
		return m, cmd
	case stateOpencodePrompt:
		if sel, ok := msg.(selectable.SelectMsg); ok {
			m.doOpencode = sel.Index == 0
			m.state = stateRunning
			return m, m.runInstallCmd()
		}
		if _, ok := msg.(selectable.CancelMsg); ok {
			m.state = stateInitPrompt // back to init
			return m, nil
		}
		updated, cmd := m.opencodePrompt.Update(msg)
		m.opencodePrompt = updated.(selectable.Model)
		return m, cmd
	}

	// Top-level state messages
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
		m.err = msg.err
		m.stderr = msg.stderr
		m.state = stateDone
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) setMode(idx int) {
	switch idx {
	case 0:
		m.mode = modeLocal
	case 1:
		m.mode = modeCloud
	case 2:
		// hybrid disabled → fallback a local (no deberia llegar aca)
		m.mode = modeLocal
	}
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
	case stateBaseURLPrompt:
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
		if len(key) == 1 {
			m.baseURL += key
			return m, nil
		}
	case stateDone:
		return m, backCmd()
	}
	return m, nil
}

// View implementa tea.Model.
func (m *Model) View() string {
	switch m.state {
	case stateWelcome:
		return m.viewWelcome()
	case stateModePrompt:
		return m.modePrompt.View()
	case stateDepCheck:
		return m.viewDepCheck()
	case stateBaseURLPrompt:
		return m.viewBaseURLPrompt()
	case stateInitPrompt:
		return m.initPrompt.View()
	case stateOpencodePrompt:
		return m.opencodePrompt.View()
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
		var status string
		switch {
		case !r.Found:
			status = styles.Fail.Render("[ MISSING ]")
		case r.Dep.MinVer != "" && !r.MinMet:
			status = styles.Warn.Render(fmt.Sprintf("[ %s < %s ]", r.Version, r.Dep.MinVer))
		default:
			status = styles.Ok.Render("[ ok ]")
		}
		s += fmt.Sprintf("  %s  %s (%s)\n", status, r.Dep.Name, r.Dep.Binary)
		if !r.Found && r.Hint != "" {
			s += fmt.Sprintf("           %s\n", styles.ItemDesc.Render(r.Hint))
		}
	}
	s += "\n"
	return s
}

func (m *Model) viewBaseURLPrompt() string {
	s := "\n  Domain server URL\n"
	s += fmt.Sprintf("  > %s\n", m.baseURL)
	s += "\n"
	s += styles.HelpText.Render("  type to edit, [enter] confirm, [esc] back") + "\n"
	return s
}

func (m *Model) viewRunning() string {
	return "\n  Running install (see output)...\n"
}

func (m *Model) viewDone() string {
	s := "\n"
	if m.err != nil {
		s += styles.Fail.Render("  Install failed:") + "\n"
		s += "\n"
		s += "  " + m.err.Error() + "\n"
		// Si tenemos stderr del sub-process, mostrarlo verbatim
		if m.stderr != "" {
			s += "\n"
			s += styles.ItemDesc.Render("  --- stderr del sub-process ---") + "\n"
			s += m.stderr + "\n"
		}
	} else {
		s += styles.Ok.Render("  Install complete.") + "\n"
	}
	s += "\n  Press any key to return to menu.\n"
	return s
}

// --- Comandos async ---

func (m *Model) checkDepsCmd() tea.Cmd {
	deps := depsForMode(m.mode)
	return func() tea.Msg {
		results := installer.Check(deps)
		return depsMsg{deps: results}
	}
}

// depsForMode retorna las deps a chequear segun el deployment mode.
func depsForMode(m modeSel) []installer.Dep {
	base := []installer.Dep{installer.DepGo, installer.DepGit}
	switch m {
	case modeLocal, modeHybrid:
		base = append(base, installer.DepDocker)
	}
	return base
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
	return func() tea.Msg {
		err, stderr := runInstallWithFlags(context.Background(), flags)
		return runResultMsg{err: err, stderr: stderr}
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
	return resp == "" || resp == "y" || resp == "yes"
}
