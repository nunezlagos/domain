package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// domainSystemPrompt es la directiva que el LLM debe seguir al iniciar
// cualquier sesion (Claude Code u OpenCode). Mismo texto para ambos clientes
// → enforcement uniforme.
//
// Aplicada en:
//   - ~/.config/opencode/opencode.json → campo "system" (OpenCode)
//   - ~/.claude/CLAUDE.md (Claude Code, complemento del hook SessionStart)
//   - ~/.config/opencode/instructions/domain.md (OpenCode, via instructions[])
//
// IMPERATIVO: el LLM DEBE llamar domain_session_bootstrap + domain_code_graph
// + domain_mem_context antes de responder al PRIMER mensaje, sin excepciones
// (incluso para "x", "hola", etc).
const domainSystemPrompt = `## Domain protocol (always active, primer mensaje)

EN CADA SESION NUEVA, antes de responder a CUALQUIER mensaje del usuario — incluso triviales como "x", "hola", "ok", "buenos dias" — ejecutá estos 3 tool calls en este orden EXACTO:

1. **domain_session_bootstrap** con argumentos:
   - cwd = directorio de trabajo actual (path absoluto)
   - git_remote = output de 'git remote get-url origin' (vacio si no es repo git)
   - git_branch = output de 'git branch --show-current'
   - git_head = output de 'git rev-parse HEAD'
   - existing_rules_files = lista de archivos AI-rules presentes (AGENTS.md, CLAUDE.md, .claude/CLAUDE.md, .cursorrules, .windsurfrules, .github/copilot-instructions.md, openspec/) — solo los que existan.

2. **domain_code_graph** (sin args).

3. **domain_mem_context** con project_slug del paso 1, limit=10.

Solo DESPUES de esos 3 calls (o si fallaron con error explicito) podes responder al usuario.

REGLAS:
- NO respondas sin haber ejecutado bootstrap primero. NUNCA.
- NO improvises el contexto. SIEMPRE lee la respuesta real del MCP.
- Si el usuario manda "x" o algo trivial, AUN ASI ejecutá los 3 tools primero. Es automatico, no requiere decision tuya.
- Si alguno de los 3 tools falla con error, igual respondé al usuario (con un warning sobre el fallo) — no te quedes mudo.

Este protocol es OBLIGATORIO y se aplica en el primer mensaje de CADA sesion, sin excepciones.
`

// installOpencodeSystemPrompt agrega/actualiza el campo "system" en
// ~/.config/opencode/opencode.json con domainSystemPrompt. Idempotente.
func installOpencodeSystemPrompt() {
	home, err := os.UserHomeDir()
	if err != nil {
		warnL("no pude resolver HOME: " + err.Error())
		return
	}
	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		warnL("no se pudo leer " + cfgPath + " (opencode no instalado?): " + err.Error())
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		warnL(cfgPath + " corrupto, no modifico system prompt")
		return
	}

	if existing, isStr := cfg["system"].(string); isStr {
		if strings.Contains(existing, "domain_session_bootstrap") && strings.Contains(existing, "primer mensaje") {
			ok("opencode system prompt ya contiene el protocolo (no duplico)")
			return
		}
	}

	cfg["system"] = domainSystemPrompt
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		warnL("marshal opencode.json: " + err.Error())
		return
	}
	if err := os.WriteFile(cfgPath, out, 0o600); err != nil {
		warnL("write opencode.json: " + err.Error())
		return
	}
	ok("opencode system prompt instalado en " + cfgPath)
	ok("→ opencode inyectara el protocolo en cada sesion como system prompt")
}