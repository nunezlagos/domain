package main

import (
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
	{Event: "Stop", Script: "domain-stop.sh", Timeout: 15},
	// REQ-54 issue-54.7: gate SDD-para-código. PostToolUse marca flow activo
	// cuando el agente orquesta; PreToolUse intercepta ediciones sin flow
	// (ask en modo normal, deny en modos automáticos).
	{Event: "PostToolUse", Script: "domain-post-orchestrate.sh", Timeout: 10,
		Matcher: "mcp__domain-mcp__domain_(orchestrate|flow_status|orchestrate_phase_result|orchestrate_confirm)"},
	{Event: "PreToolUse", Script: "domain-pre-edit.sh", Timeout: 10,
		Matcher: "Edit|Write|NotebookEdit|Bash"},
	// PostToolUse tras Bash: captura el resultado de correr tests/suite para
	// que el auto-behavior de domain lo observe (SUGGEST-ONLY, best-effort).
	{Event: "PostToolUse", Script: "domain-post-test.sh", Timeout: 10,
		Matcher: "Bash"},
}

// installClaudeSessionStartHook registra los lifecycle hooks de domain en
// ~/.claude/settings.json. Idempotente: si un hook ya está, no duplica. Si
// settings.json no existe (instalación limpia), lo crea. Los scripts deben
// existir en ~/.local/share/domain/hooks/ (los instala install-curl.sh /
// el install canónico); si falta alguno, se avisa y se salta ese hook.
func installClaudeSessionStartHook() {
	home, err := os.UserHomeDir()
	if err != nil {
		warnL("no pude resolver HOME para instalar hooks: " + err.Error())
		return
	}
	hooksDir := filepath.Join(home, ".local", "share", "domain", "hooks")
	settingsPath := claudeSettingsPath(home)
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
		hookPath := filepath.Join(hooksDir, spec.Script)
		if _, err := os.Stat(hookPath); err != nil {
			warnL("hook script no encontrado en " + hookPath + " (re-corré el install canónico para instalarlo)")
			continue
		}
		if claudeHookRegistered(hooks, spec.Event, hookPath) {
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
func claudeHookRegistered(hooks map[string]any, event, hookPath string) bool {
	arr, ok := hooks[event].([]any)
	if !ok {
		return false
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
				return true
			}
		}
	}
	return false
}

func toArray(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{}
}
