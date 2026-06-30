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
func domainBlock() string {
	body := strings.TrimRight(string(claudeGlobalMD), "\n")
	return domainBlockStart + "\n" + body + "\n" + domainBlockEnd
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
	block := domainBlock()
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
	return strings.Contains(content, domainBlock())
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
// Además, si existe la config global de OpenCode, registra el mismo archivo
// como instruction global de OpenCode (ver installOpencodeGlobalInstruction).
func installGlobalInstructions(paths Paths, home, timestamp string) error {
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

	return installOpencodeGlobalInstruction(paths, timestamp)
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
