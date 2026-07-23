package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Marcadores que delimitan la sección gestionada por domain dentro del
// CLAUDE.md global del usuario. Todo lo de afuera es del usuario y NO se
// toca; en upgrades solo se reemplaza lo que está entre los marcadores.
const (
	domainBlockStart = "<!-- domain:start -->"
	domainBlockEnd   = "<!-- domain:end -->"
)

// domainBlock arma la sección gestionada: marcadores + el template embebido.
// Se usa para el archivo dedicado de OpenCode (que se lee completo). Usa el
// template propio de OpenCode: como OpenCode NO tiene hook SessionStart, su
// protocolo de PRIMER MENSAJE instruye ejecutar el bootstrap manualmente,
// a diferencia del template de Claude Code (donde el hook ya lo ejecutó).
func domainBlock() string {
	body := strings.TrimRight(string(opencodeGlobalMD), "\n")
	return domainBlockStart + "\n" + body + "\n" + domainBlockEnd
}

// domainFileBody es el contenido del archivo dedicado ~/.claude/domain.md
// (100% gestionado por domain, sin marcadores: el archivo es todo de domain).
func domainFileBody() string {
	return strings.TrimRight(string(claudeGlobalMD), "\n") + "\n"
}

// domainImportBlock es lo que va DENTRO de los marcadores en ~/.claude/CLAUDE.md:
// un @import al archivo dedicado domain.md. Mantiene el contenido real FUERA del
// CLAUDE.md (en domain.md, nombre propio) para que excluir CLAUDE.md/AGENTS.md de
// proyecto NO neutralice el bloque global de domain. issue-54.1.
func domainImportBlock() string {
	return domainBlockStart + "\n@domain.md\n" + domainBlockEnd
}

// claudeDomainMdPath es ~/.claude/domain.md (archivo dedicado de domain).
func claudeDomainMdPath(home string) string {
	return filepath.Join(home, ".claude", "domain.md")
}

// claudePersonaMdPath es ~/.claude/persona.md: la personalidad del agente,
// archivo dedicado y editable. domain.md lo referencia con `@persona.md`, así
// que se puede editar el tono sin tocar el protocolo. 100% gestionado.
func claudePersonaMdPath(home string) string {
	return filepath.Join(home, ".claude", "persona.md")
}

// personaFileBody es el contenido de ~/.claude/persona.md (template embebido).
func personaFileBody() string {
	return strings.TrimRight(string(claudePersonaMD), "\n") + "\n"
}

// upsertDomainBlock devuelve el contenido con la sección domain insertada o
// actualizada, preservando todo lo que el usuario tenga afuera de los
// marcadores. Reglas:
//   - si existe el par de marcadores, reemplaza SOLO lo de adentro;
//   - si el contenido está vacío, escribe solo el bloque;
//   - si no existe, agrega el bloque al final separado por una línea en blanco.
//
// Es idempotente: aplicarlo dos veces sobre su propia salida no duplica ni
// muta nada.
func upsertDomainBlock(content string) string {
	block := domainImportBlock()
	start := strings.Index(content, domainBlockStart)
	end := strings.Index(content, domainBlockEnd)
	if start != -1 && end != -1 && end > start {
		before := content[:start]
		after := content[end+len(domainBlockEnd):]
		return before + block + after
	}
	if strings.TrimSpace(content) == "" {
		return block + "\n"
	}
	return strings.TrimRight(content, "\n") + "\n\n" + block + "\n"
}

// hasUpToDateDomainBlock indica si el contenido ya tiene la sección domain con
// el bloque actual (para reportar no-op vs upgrade).
func hasUpToDateDomainBlock(content string) bool {
	return strings.Contains(content, domainImportBlock())
}

// claudeGlobalPath es el CLAUDE.md global de Claude Code (~/.claude/CLAUDE.md):
// instrucciones que aplican a todos los proyectos del usuario.
func claudeGlobalPath(home string) string {
	return filepath.Join(home, ".claude", "CLAUDE.md")
}

// installGlobalInstructions escribe el bloque de precedencia de domain en
// ~/.claude/CLAUDE.md dentro de una sección marcada, sin pisar el contenido del
// usuario, con backup previo. Stdlib puro. Idempotente.
//
// La instruction global de OpenCode se escribe aparte, en el cluster post-Apply
// de runInstall (installOpencodeGlobalInstruction), porque su directorio de
// config recién existe después de Apply (DOMAINSERV-101).
func installGlobalInstructions(home, timestamp string) error {
	// 0. Persona en archivo dedicado ~/.claude/persona.md (editable por el
	//    usuario). domain.md la referencia con @persona.md. Backup si cambia.
	personaPath := claudePersonaMdPath(home)
	persona := personaFileBody()
	if cur, existed, err := readIfExists(personaPath); err != nil {
		return fmt.Errorf("read %s: %w", personaPath, err)
	} else if !existed || cur != persona {
		if err := os.MkdirAll(filepath.Dir(personaPath), 0o755); err != nil {
			return fmt.Errorf("mkdir ~/.claude: %w", err)
		}
		if existed {
			if _, err := backupIfExists(personaPath, timestamp); err != nil {
				return fmt.Errorf("backup persona.md: %w", err)
			}
		}
		if err := os.WriteFile(personaPath, []byte(persona), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", personaPath, err)
		}
	}

	// 1. Contenido real de domain en archivo dedicado ~/.claude/domain.md
	//    (nombre propio, 100% gestionado: se sobreescribe entero). Backup si cambia.
	domainPath := claudeDomainMdPath(home)
	body := domainFileBody()
	if cur, existed, err := readIfExists(domainPath); err != nil {
		return fmt.Errorf("read %s: %w", domainPath, err)
	} else if !existed || cur != body {
		if err := os.MkdirAll(filepath.Dir(domainPath), 0o755); err != nil {
			return fmt.Errorf("mkdir ~/.claude: %w", err)
		}
		if existed {
			if _, err := backupIfExists(domainPath, timestamp); err != nil {
				return fmt.Errorf("backup domain.md: %w", err)
			}
		}
		if err := os.WriteFile(domainPath, []byte(body), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", domainPath, err)
		}
	}

	// 2. ~/.claude/CLAUDE.md: solo el bloque marcado con @domain.md, preservando
	//    el contenido del usuario fuera de los marcadores.
	path := claudeGlobalPath(home)
	content, existed, err := readIfExists(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !existed || !hasUpToDateDomainBlock(content) {
		newContent := upsertDomainBlock(content)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("mkdir ~/.claude: %w", err)
		}
		if existed {
			if _, err := backupIfExists(path, timestamp); err != nil {
				return fmt.Errorf("backup: %w", err)
			}
		}
		if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	// DOMAINSERV-101: la instruction global de OpenCode se escribe en el cluster
	// post-Apply de runInstall (main.go), no acá — en install fresco
	// ~/.config/opencode aún no existe en este punto y el write sería un no-op.
	return nil
}

// installOpencodeGlobalInstruction escribe el bloque de precedencia como
// archivo de instrucción global de OpenCode (~/.config/opencode/instructions/
// domain.md) y lo referencia en el array "instructions" del opencode.json
// global. OpenCode no usa marcadores: el archivo es 100% gestionado por domain,
// así que se sobreescribe completo (idempotente). El array "instructions" se
// upsertea sin duplicar y preservando otras entradas del usuario.
//
// Si OpenCode no está presente (no existe el dir de config), es un no-op.
func installOpencodeGlobalInstruction(paths Paths, timestamp string) error {
	if !dirExists(paths.OpencodeDir) {
		return nil
	}

	instrDir := filepath.Join(paths.OpencodeDir, "instructions")
	instrPath := filepath.Join(instrDir, "domain.md")
	if err := os.MkdirAll(instrDir, 0o755); err != nil {
		return fmt.Errorf("mkdir opencode instructions: %w", err)
	}
	if err := os.WriteFile(instrPath, []byte(domainBlock()+"\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", instrPath, err)
	}

	// Referencia relativa al dir de config de opencode, para que el JSON sea
	// portable y consistente con cómo opencode resuelve instructions.
	ref := "instructions/domain.md"
	m, err := loadOrEmptyJSON(paths.OpencodeMCP)
	if err != nil {
		return err
	}
	// Solo respaldamos + escribimos si realmente hay que mutar el JSON, para
	// que re-ejecutar el instalador sea idempotente (no acumule backups).
	if upsertStringInArray(m, "instructions", ref) {
		if _, err := backupIfExists(paths.OpencodeMCP, timestamp); err != nil {
			return fmt.Errorf("backup opencode.json: %w", err)
		}
		return writeJSON(paths.OpencodeMCP, m)
	}
	return nil
}

// upsertStringInArray agrega value al array de strings bajo key si no está
// presente. Devuelve true si modificó el map (hay que persistir). Preserva
// elementos existentes y tolera arrays con tipos mixtos.
func upsertStringInArray(m map[string]any, key, value string) bool {
	raw, ok := m[key].([]any)
	if !ok {
		// Si había un valor escalar previo (ej. un string suelto), lo envolvemos
		// en array en vez de pisarlo — evita data loss (issue-54.1 review).
		if existing, isStr := m[key].(string); isStr && existing != "" {
			raw = []any{existing}
		} else {
			raw = []any{}
		}
	}
	for _, e := range raw {
		if s, ok := e.(string); ok && s == value {
			return false
		}
	}
	m[key] = append(raw, value)
	return true
}

// readIfExists lee path. Si no existe, devuelve ("", false, nil).
func readIfExists(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}
