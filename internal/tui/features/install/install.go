// Package install — TUI feature para `domain install` (HU-01.11 + 01.13,
// rediseño 2026-06-11: config completa primero, instalación automática
// verbosa después).
//
// Flow:
//   1. welcome
//   2. modePrompt:   (•) local / ( ) cloud / [-] hybrid  + Continuar
//   3. depCheck:     resultados de go/git/[docker]; bloquea si falta algo
//   4. portPrompt:   (local) puerto sugerido libre, editable
//      dsnPrompt:    (cloud) DSN de Postgres
//   5. initPrompt:   importar configs .md a la BD  + Continuar
//   6. agentsPrompt: MULTI [X] opencode / [ ] claude-code + Continuar
//   7. summary:      toda la config elegida → [ Instalar ]
//   8. running:      sub-process con output en vivo (verboso)
//   9. done
//
// Regla de navegación: enter/espacio ELIGE, nunca avanza. Se avanza con
// el botón Continuar de cada vista (o enter en inputs de texto).

package install

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

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
	statePortPrompt
	stateDSNPrompt
	stateInitPrompt
	stateAgentsPrompt
	stateSummary
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

// agentLabels índices del multi-select de agentes.
var agentLabels = []string{"opencode", "claude-code"}

// Model bubbletea para la feature install.
type Model struct {
	state    state
	platform installer.Platform
	deps     []installer.CheckResult
	depsMissing bool

	// Config elegida
	mode   modeSel
	port   string
	dsn    string
	doInit bool
	agents []string

	err    error
	stderr string

	// Output en vivo del sub-process (running)
	lines []string
	runCh chan tea.Msg

	// sub-models
	modePrompt   selectable.Model
	initPrompt   selectable.Model
	agentsPrompt selectable.Model
}

func New() *Model {
	return &Model{
		state:  stateWelcome,
		port:   suggestPort(8000),
		doInit: true,
		agents: []string{"opencode"},
		modePrompt: selectable.New("¿Dónde van a vivir los servicios?", []selectable.Item{
			{Label: "local", Description: "Todo en esta máquina: Postgres + S3 + SMTP via Docker"},
			{Label: "cloud", Description: "Servicios existentes: pegás la URL de tu Postgres (DSN)"},
			{Label: "hybrid", Description: "Mezcla por servicio (todavía no disponible)", Disabled: true},
		}),
		initPrompt: selectable.New("¿Importar tus configs de agentes a la base de datos?", []selectable.Item{
			{Label: "sí, importar", Description: "Copia CLAUDE.md, .claude/** y .opencode/** a la BD como respaldo versionado"},
			{Label: "no, saltear", Description: "Podés hacerlo después con: domain init"},
		}),
		agentsPrompt: selectable.NewMulti("¿En qué agentes instalar el MCP server?", []selectable.Item{
			{Label: "opencode", Description: "Agrega 'domain' a opencode.json del proyecto"},
			{Label: "claude-code", Description: "Agrega 'domain' a .mcp.json del proyecto"},
		}, []int{0}),
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

type lineMsg string

type runResultMsg struct {
	err    error
	stderr string
}

// Update implementa tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Sub-prompts: delegar primero.
	switch m.state {
	case stateModePrompt:
		if sel, ok := msg.(selectable.SelectMsg); ok {
			m.setMode(sel.Index)
			m.state = stateDepCheck
			return m, m.checkDepsCmd()
		}
		if _, ok := msg.(selectable.CancelMsg); ok {
			return m, backCmd()
		}
		updated, cmd := m.modePrompt.Update(msg)
		m.modePrompt = updated.(selectable.Model)
		return m, cmd
	case stateInitPrompt:
		if sel, ok := msg.(selectable.SelectMsg); ok {
			m.doInit = sel.Index == 0
			m.state = stateAgentsPrompt
			return m, nil
		}
		if _, ok := msg.(selectable.CancelMsg); ok {
			if m.mode == modeCloud {
				m.state = stateDSNPrompt
			} else {
				m.state = statePortPrompt
			}
			return m, nil
		}
		updated, cmd := m.initPrompt.Update(msg)
		m.initPrompt = updated.(selectable.Model)
		return m, cmd
	case stateAgentsPrompt:
		if sel, ok := msg.(selectable.MultiSelectMsg); ok {
			m.agents = m.agents[:0]
			for _, idx := range sel.Indices {
				if idx >= 0 && idx < len(agentLabels) {
					m.agents = append(m.agents, agentLabels[idx])
				}
			}
			m.state = stateSummary
			return m, nil
		}
		if _, ok := msg.(selectable.CancelMsg); ok {
			m.state = stateInitPrompt
			return m, nil
		}
		updated, cmd := m.agentsPrompt.Update(msg)
		m.agentsPrompt = updated.(selectable.Model)
		return m, cmd
	}

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
		m.depsMissing = false
		for _, r := range msg.deps {
			if !r.Found || (r.Dep.MinVer != "" && !r.MinMet) {
				m.depsMissing = true
			}
		}
		return m, nil
	case lineMsg:
		m.lines = append(m.lines, string(msg))
		if len(m.lines) > 200 {
			m.lines = m.lines[len(m.lines)-200:]
		}
		return m, waitForRunMsg(m.runCh)
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
	default:
		m.mode = modeLocal
	}
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.state {
	case stateWelcome:
		if key == "esc" || key == "q" {
			return m, backCmd()
		}
	case stateDepCheck:
		switch key {
		case "enter", " ":
			if m.depsMissing {
				return m, nil // bloqueado hasta instalar la dep
			}
			if m.mode == modeCloud {
				m.state = stateDSNPrompt
			} else {
				m.state = statePortPrompt
			}
			return m, nil
		case "esc":
			m.state = stateModePrompt
			return m, nil
		}
	case statePortPrompt:
		switch {
		case key == "enter":
			if m.port == "" {
				m.port = suggestPort(8000)
			}
			m.state = stateInitPrompt
			return m, nil
		case key == "esc":
			m.state = stateDepCheck
			return m, nil
		case key == "backspace":
			if len(m.port) > 0 {
				m.port = m.port[:len(m.port)-1]
			}
			return m, nil
		case len(key) == 1 && key >= "0" && key <= "9" && len(m.port) < 5:
			m.port += key
			return m, nil
		}
	case stateDSNPrompt:
		switch {
		case key == "enter":
			if strings.TrimSpace(m.dsn) == "" {
				return m, nil // DSN obligatoria en cloud
			}
			m.state = stateInitPrompt
			return m, nil
		case key == "esc":
			m.state = stateDepCheck
			return m, nil
		case key == "backspace":
			if len(m.dsn) > 0 {
				m.dsn = m.dsn[:len(m.dsn)-1]
			}
			return m, nil
		case len(key) == 1:
			m.dsn += key
			return m, nil
		}
	case stateSummary:
		switch key {
		case "enter":
			m.state = stateRunning
			m.lines = nil
			return m, m.startInstallCmd()
		case "esc":
			m.state = stateAgentsPrompt
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
	case statePortPrompt:
		return m.viewPortPrompt()
	case stateDSNPrompt:
		return m.viewDSNPrompt()
	case stateInitPrompt:
		return m.initPrompt.View()
	case stateAgentsPrompt:
		return m.agentsPrompt.View()
	case stateSummary:
		return m.viewSummary()
	case stateRunning:
		return m.viewRunning()
	case stateDone:
		return m.viewDone()
	}
	return ""
}

// viewWelcome es transitoria: detectPlatformCmd resuelve en ms y pasa
// directo al modePrompt ("pum, entramos al instalador").
func (m *Model) viewWelcome() string {
	s := "\n"
	s += styles.Title.Render("  Domain Install") + "\n\n"
	s += styles.ItemDesc.Render("  Detectando plataforma...") + "\n"
	return s
}

func (m *Model) viewDepCheck() string {
	s := "\n  " + styles.Title.Render("Dependencias") + "\n\n"
	if len(m.deps) == 0 {
		return s + styles.ItemDesc.Render("  Chequeando dependencias...") + "\n"
	}
	for _, r := range m.deps {
		var status string
		switch {
		case !r.Found:
			status = styles.Fail.Render("[✗]")
		case r.Dep.MinVer != "" && !r.MinMet:
			status = styles.Warn.Render(fmt.Sprintf("[!] %s < %s", r.Version, r.Dep.MinVer))
		default:
			status = styles.Ok.Render("[✓]")
		}
		s += fmt.Sprintf("  %s  %s (%s)\n", status, r.Dep.Name, r.Dep.Binary)
		if !r.Found && r.Hint != "" {
			s += "       " + styles.ItemDesc.Render(r.Hint) + "\n"
		}
	}
	s += "\n"
	if m.depsMissing {
		s += styles.Fail.Render("  Faltan dependencias.") +
			styles.ItemDesc.Render(" Instalalas y volvé a entrar.") + "\n"
		s += styles.HelpText.Render("  [esc] volver") + "\n"
	} else {
		s += "  > " + styles.ButtonFocused.Render("[ Continuar ]") + "\n\n"
		s += styles.HelpText.Render("  [enter] continuar   [esc] volver") + "\n"
	}
	return s
}

func (m *Model) viewPortPrompt() string {
	s := "\n  " + styles.Title.Render("Puerto del server") + "\n\n"
	s += styles.ItemDesc.Render("  El server HTTP de domain escucha en localhost. Sugerimos un") + "\n"
	s += styles.ItemDesc.Render("  puerto libre (8000 si está disponible).") + "\n\n"
	s += "  Puerto: " + styles.Accent.Render(m.port) + styles.Prompt.Render("▌") + "\n\n"
	s += styles.HelpText.Render("  [0-9] editar   [backspace] borrar   [enter] continuar   [esc] volver") + "\n"
	return s
}

func (m *Model) viewDSNPrompt() string {
	s := "\n  " + styles.Title.Render("Base de datos (cloud)") + "\n\n"
	s += styles.ItemDesc.Render("  Pegá la URL de tu Postgres:") + "\n"
	s += styles.ItemDesc.Render("  postgres://user:pass@host:5432/domain?sslmode=require") + "\n\n"
	s += "  DSN: " + styles.Accent.Render(m.dsn) + styles.Prompt.Render("▌") + "\n\n"
	s += styles.HelpText.Render("  [enter] continuar   [esc] volver") + "\n"
	return s
}

func (m *Model) viewSummary() string {
	yesNo := func(b bool) string {
		if b {
			return styles.Ok.Render("sí")
		}
		return styles.ItemDesc.Render("no")
	}
	agents := strings.Join(m.agents, ", ")
	if agents == "" {
		agents = "ninguno"
	}
	s := "\n  " + styles.Title.Render("Resumen — revisá antes de instalar") + "\n\n"
	s += fmt.Sprintf("  Modo:            %s\n", styles.Accent.Render(m.mode.String()))
	if m.mode == modeCloud {
		s += fmt.Sprintf("  DSN:             %s\n", styles.Accent.Render(redactDSN(m.dsn)))
	} else {
		s += fmt.Sprintf("  Puerto:          %s\n", styles.Accent.Render(m.port))
	}
	s += fmt.Sprintf("  Base URL:        %s\n", styles.Accent.Render(m.baseURL()))
	s += fmt.Sprintf("  Importar .md:    %s\n", yesNo(m.doInit))
	s += fmt.Sprintf("  Agentes MCP:     %s\n", styles.Accent.Render(agents))
	s += "\n  > " + styles.ButtonFocused.Render("[ Instalar ]") + "\n\n"
	s += styles.HelpText.Render("  [enter] instalar   [esc] volver") + "\n"
	return s
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// viewRunning muestra UNA sola línea con el paso actual (estilo ptools):
//
//	⠹ [5/11] Applying migrations — schema up to date
//
// El spinner avanza con cada línea de output recibida. El log completo
// queda en m.lines para el detalle en done.
func (m *Model) viewRunning() string {
	s := "\n  " + styles.Title.Render("Instalando") + "\n\n"

	step, detail := lastStepAndDetail(m.lines)
	if step == "" {
		s += "  " + styles.Accent.Render(spinnerFrames[0]) +
			" " + styles.ItemDesc.Render("arrancando...") + "\n"
		return s
	}
	frame := spinnerFrames[len(m.lines)%len(spinnerFrames)]
	line := "  " + styles.Accent.Render(frame) + " " + styles.ItemTitle.Render(step)
	if detail != "" {
		line += styles.ItemDesc.Render(" — " + detail)
	}
	s += line + "\n"
	return s
}

// lastStepAndDetail extrae del output el último step "[N/M] Title" y,
// si ya llegó su resultado (✓/·/⚠/✗ summary), el detalle.
func lastStepAndDetail(lines []string) (step, detail string) {
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "[") {
			return t, detail
		}
		if detail == "" {
			r := []rune(t)
			if len(r) > 0 && (r[0] == '✓' || r[0] == '·' || r[0] == '⚠' || r[0] == '✗') {
				detail = strings.TrimSpace(string(r[1:]))
			}
		}
	}
	return "", detail
}

func (m *Model) viewDone() string {
	s := "\n"
	if m.err != nil {
		s += styles.Fail.Render("  ✗ La instalación falló") + "\n\n"
		s += "  " + m.err.Error() + "\n"
		if m.stderr != "" {
			s += "\n" + styles.ItemDesc.Render("  --- stderr del sub-proceso ---") + "\n"
			s += m.stderr + "\n"
		}
	} else {
		s += styles.Ok.Render("  ✓ Instalación completa") + "\n\n"
		// Recap final con el output completo reciente
		tail := m.lines
		if len(tail) > 10 {
			tail = tail[len(tail)-10:]
		}
		for _, line := range tail {
			s += "  " + styles.ItemDesc.Render(line) + "\n"
		}
	}
	s += "\n" + styles.HelpText.Render("  [cualquier tecla] volver al menú") + "\n"
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

// baseURL deriva la URL del server según el modo.
func (m *Model) baseURL() string {
	if m.mode == modeCloud {
		return "http://localhost:8000"
	}
	port := m.port
	if port == "" {
		port = "8000"
	}
	return "http://localhost:" + port
}

// installFlags arma los flags del sub-process según la config elegida.
func (m *Model) installFlags() []string {
	flags := []string{
		"--mode", m.mode.String(),
		"--base-url", m.baseURL(),
		"--non-interactive",
		"--agents", strings.Join(m.agents, ","),
	}
	if !m.doInit {
		flags = append(flags, "--no-init")
	}
	if m.mode == modeCloud && m.dsn != "" {
		flags = append(flags, "--dsn", m.dsn)
	}
	return flags
}

// startInstallCmd lanza el sub-process con streaming de líneas hacia la TUI.
func (m *Model) startInstallCmd() tea.Cmd {
	flags := m.installFlags()
	ch := make(chan tea.Msg, 64)
	m.runCh = ch
	go func() {
		err, stderr := runInstallStreaming(context.Background(), flags, func(line string) {
			ch <- lineMsg(line)
		})
		ch <- runResultMsg{err: err, stderr: stderr}
	}()
	return waitForRunMsg(ch)
}

// waitForRunMsg espera el próximo mensaje del canal de streaming.
func waitForRunMsg(ch chan tea.Msg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg { return <-ch }
}

// suggestPort sugiere el puerto para el server:
//   - si en `start` YA responde un domain server (/health 200), reusa ese
//     puerto — es nuestro propio service corriendo, no un conflicto
//   - si está libre, lo sugiere
//   - si lo ocupa otra cosa, busca el siguiente libre (start..start+20)
func suggestPort(start int) string {
	for p := start; p < start+20; p++ {
		if isDomainServer(p) {
			return fmt.Sprintf("%d", p)
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			_ = ln.Close()
			return fmt.Sprintf("%d", p)
		}
	}
	return fmt.Sprintf("%d", start)
}

// isDomainServer chequea si en el puerto responde /health con 200
// (nuestro server corriendo, e.g. via domain.service).
func isDomainServer(port int) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// redactDSN oculta el password de una DSN para mostrarla en el summary.
func redactDSN(dsn string) string {
	at := strings.Index(dsn, "@")
	scheme := strings.Index(dsn, "://")
	if at < 0 || scheme < 0 || at < scheme {
		return dsn
	}
	creds := dsn[scheme+3 : at]
	if colon := strings.Index(creds, ":"); colon >= 0 {
		return dsn[:scheme+3] + creds[:colon] + ":***" + dsn[at:]
	}
	return dsn
}

// --- helpers ---

func backCmd() tea.Cmd {
	return func() tea.Msg { return menu.BackMsg{} }
}
