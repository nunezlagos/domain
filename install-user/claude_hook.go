package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// claudeHookSpec define un hook de Claude Code a registrar en
// ~/.claude/settings.json: evento + script (en ~/.local/share/domain/hooks/)
// + timeout opcional en segundos (0 = default del evento).
type claudeHookSpec struct {
	Event   string
	Script  string
	Timeout int
	// Matcher filtra el hook por tool name (regex) en eventos que lo soportan
	// (PreToolUse/PostToolUse). Vacío = sin matcher (el hook corre siempre).
	Matcher string
	// Subcomando, si está, hace que el hook se registre como `<binario> hook <subcomando>`
	// en vez del path del .sh (DOMAINSERV-273). Es lo que permite portarlos de a uno: los
	// que todavía no tienen subcomando siguen registrándose como script.
	//
	// Se usa el path ABSOLUTO del binario y no su nombre: depender del PATH ataría este
	// trabajo a DOMAINSERV-263, que es justamente el ticket de que el binario no está ahí.
	Subcomando string
}

// claudeHooks es el set de lifecycle hooks de domain (REQ-54):
//   - SessionStart: pre-carga bootstrap + mem context ANTES del
//     primer prompt (inyecta additionalContext).
//   - UserPromptSubmit: captura CADA prompt vía domain_prompt_capture y guarda
//     el prompt_id por session (determinista, no depende del LLM).
//   - Stop: cierra el turno vía domain_turn_complete con el prompt_id guardado.
//
// Timeouts cortos en los de lifecycle: son best-effort y no deben demorar la
// sesión si el VPS anda lento.
var claudeHooks = []claudeHookSpec{
	{Event: "SessionStart", Script: "domain-session-start.sh"},
	{Event: "UserPromptSubmit", Script: "domain-user-prompt.sh", Timeout: 15},
	// DOMAINSERV-273: primero de los 7 en portarse a Go. Se eligió por ser el más chico (73
	// líneas) para validar el patrón completo —subcomando, registro, tests, distribución—
	// antes de tocar domain-pre-edit.sh, que son 1.593 y es el gate que autoriza las
	// ediciones con las que se lo arreglaría.
	{Event: "Stop", Script: "domain-stop.sh", Timeout: 15, Subcomando: "stop"},
	// REQ-54 issue-54.7: gate SDD-para-código. PostToolUse marca flow activo
	// cuando el agente orquesta; PreToolUse intercepta ediciones sin flow
	// (ask en modo normal, deny en modos automáticos).
	// DOMAINSERV-115: flow_status y flow_grant_token TIENEN que estar en el
	// matcher. El token dura 30 min y sin ellas no hay forma de renovar el
	// marker: el flow sigue vivo server-side pero el gate deniega, y la única
	// salida era re-orquestar — que sdd-auto-trigger prohíbe.
	{Event: "PostToolUse", Script: "domain-post-orchestrate.sh", Timeout: 10,
		Matcher: "mcp__domain-mcp__domain_(orchestrate|orchestrate_phase_result|orchestrate_confirm|flow_cancel|flow_status|flow_grant_token)"},
	{Event: "PreToolUse", Script: "domain-pre-edit.sh", Timeout: 10,
		Matcher: "Edit|Write|NotebookEdit|Bash"},
	// PostToolUse tras Bash: captura el resultado de correr tests/suite para
	// que el auto-behavior de domain lo observe (SUGGEST-ONLY, best-effort).
	{Event: "PostToolUse", Script: "domain-post-test.sh", Timeout: 10,
		Matcher: "Bash"},
}

// comandoDelHook devuelve el `command` con el que se registra un hook: el binario con su
// subcomando si ya está portado a Go, o el path del script si todavía no (DOMAINSERV-273).
//
// El .sh se sigue exigiendo en disco aunque el hook esté portado. No es redundancia: el
// instalador lo instala igual, y tenerlo permite volver atrás editando settings.json si el
// subcomando fallara en una plataforma donde no lo pudimos probar.
func comandoDelHook(spec claudeHookSpec, hooksDir string) (string, error) {
	hookPath := filepath.Join(hooksDir, spec.Script)
	if _, err := os.Stat(hookPath); err != nil {
		return "", fmt.Errorf("hook script no encontrado en %s (re-corré el install canónico para instalarlo)", hookPath)
	}
	if spec.Subcomando == "" {
		return hookPath, nil
	}
	binario, err := os.Executable()
	if err != nil {
		// sin path del binario el registro quedaría apuntando a un nombre suelto, que depende
		// del PATH — exactamente lo que este diseño evita
		return "", fmt.Errorf("no se pudo resolver el path del binario para el hook %s: %v", spec.Event, err)
	}
	return binario + " hook " + spec.Subcomando, nil
}

// installClaudeSessionStartHook registra los lifecycle hooks de domain en
// ~/.claude/settings.json. Idempotente: si un hook ya está, no duplica. Si
// settings.json no existe (instalación limpia), lo crea. Los scripts deben
// existir en ~/.local/share/domain/hooks/ (los instala install-curl.sh /
// el install canónico); si falta alguno, se avisa y se salta ese hook.
// DOMAINSERV-279: recibe el config dir del perfil. Los SCRIPTS son compartidos
// (HooksDirDelSistema, y se registran con path absoluto), pero el REGISTRO vive en el
// settings.json de cada perfil: un perfil sin registro no ejecuta ningún hook.
func installClaudeSessionStartHook(configDir string) {
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			warnL("no pude resolver HOME para instalar hooks: " + err.Error())
			return
		}
		configDir = filepath.Join(home, ".claude")
	}
	hooksDir := HooksDirDelSistema()
	settingsPath := claudeSettingsPathIn(configDir)
	cfg, err := loadOrEmptyJSON(settingsPath)
	if err != nil {
		warnL(settingsPath + " corrupto, hooks no instalados: " + err.Error())
		return
	}

	hooks, _ := cfg["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		cfg["hooks"] = hooks
	}

	changed := false
	for _, spec := range claudeHooks {
		hookPath, err := comandoDelHook(spec, hooksDir)
		if err != nil {
			warnL(err.Error())
			continue
		}
		if exists, updated := reconcileClaudeHook(hooks, spec.Event, hookPath, spec.Matcher); exists {
			if updated {
				changed = true
				ok("hook " + spec.Event + " matcher reconciliado: " + hookPath)
			}
			continue
		}
		entry := map[string]any{
			"type":    "command",
			"command": hookPath,
		}
		if spec.Timeout > 0 {
			entry["timeout"] = spec.Timeout
		}
		newEntry := map[string]any{
			"hooks": []any{entry},
		}
		if spec.Matcher != "" {
			newEntry["matcher"] = spec.Matcher
		}
		hooks[spec.Event] = append(toArray(hooks[spec.Event]), newEntry)
		changed = true
		ok("hook " + spec.Event + " instalado: " + hookPath)
	}

	if !changed {
		return
	}
	if _, err := backupIfExists(settingsPath, Timestamp()); err != nil {
		warnL("backup settings.json: " + err.Error())
		return
	}
	if err := writeJSON(settingsPath, cfg); err != nil {
		warnL("write settings.json: " + err.Error())
		return
	}
	ok("→ lifecycle domain activo: bootstrap pre-prompt + captura de prompts + cierre de turnos")
}

// claudeHookRegistered indica si el evento ya tiene registrado un hook cuyo
// command sea exactamente hookPath.
// claudeHookRegistered: ¿el hook (por command) ya está en la config? Read-only
// (matcher="" → reconcileClaudeHook no muta). Lo usa el doctor.
func claudeHookRegistered(hooks map[string]any, event, hookPath string) bool {
	exists, _ := reconcileClaudeHook(hooks, event, hookPath, "")
	return exists
}

// reconcileClaudeHook busca el hook por command. Si existe, reconcilia su matcher
// al esperado (DOMAINSERV-84 idempotencia): un install previo pudo dejar un matcher
// stale y el skip-si-existe lo perpetuaba. Devuelve (existe, actualizó-el-matcher).
func reconcileClaudeHook(hooks map[string]any, event, hookPath, matcher string) (exists, updated bool) {
	arr, ok := hooks[event].([]any)
	if !ok {
		return false, false
	}
	for _, entry := range arr {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hs, ok := m["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == hookPath {
				if matcher != "" {
					if cur, _ := m["matcher"].(string); cur != matcher {
						m["matcher"] = matcher
						return true, true
					}
				}
				return true, false
			}
		}
	}
	return false, false
}

func toArray(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{}
}
