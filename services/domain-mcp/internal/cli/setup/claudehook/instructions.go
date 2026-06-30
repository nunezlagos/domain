package claudehook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nunezlagos/domain/internal/agentprotocol"
	"nunezlagos/domain/internal/cli/install"
)

// Marcadores que delimitan la sección gestionada por domain dentro del
// CLAUDE.md global del usuario. Todo lo de afuera es del usuario y NO se
// toca; solo se reemplaza lo de adentro en upgrades.
const (
	domainBlockStart = "<!-- domain:start -->"
	domainBlockEnd   = "<!-- domain:end -->"
)

// ClaudeMDPath es el CLAUDE.md global de Claude Code (~/.claude/CLAUDE.md):
// instrucciones que aplican a todos los proyectos del usuario.
func ClaudeMDPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "CLAUDE.md")
}

// domainBlock arma la sección gestionada (marcadores + Stub).
func domainBlock() string {
	return domainBlockStart + "\n" + agentprotocol.Stub + "\n" + domainBlockEnd
}

// HasUpToDateDomainBlock indica si el contenido ya tiene la sección domain
// con el Stub actual (para reportar no-op vs upgrade).
func HasUpToDateDomainBlock(content string) bool {
	return strings.Contains(content, domainBlock())
}

// upsertDomainBlock devuelve el contenido con la sección domain insertada o
// actualizada, preservando todo lo que el usuario tenga afuera de los
// marcadores. Si no existe la sección, la agrega al final.
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

// InstallGlobalInstructions escribe el Stub de domain en ~/.claude/CLAUDE.md
// dentro de una sección marcada, sin pisar el contenido del usuario. Esto da
// enforcement global en Claude Code: el protocolo aplica en cualquier repo.
// Retorna "installed" | "updated" | "already_installed" | "declined".
func InstallGlobalInstructions(autoAccept bool) (string, error) {
	path := ClaudeMDPath()
	if path == "" {
		return "", fmt.Errorf("no se pudo resolver HOME")
	}

	var content string
	existed := false
	if raw, err := os.ReadFile(path); err == nil {
		content = string(raw)
		existed = true
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	if existed && HasUpToDateDomainBlock(content) {
		return "already_installed", nil
	}

	newContent := upsertDomainBlock(content)

	if !autoAccept {
		fmt.Printf("Voy a escribir el protocolo domain en %s\n", path)
		fmt.Println("(sección marcada <!-- domain:start/end -->; tu contenido no se toca)")
		fmt.Print("Aplicar? [y/N] ")
		var resp string
		fmt.Scanln(&resp)
		if r := strings.TrimSpace(strings.ToLower(resp)); r != "y" && r != "yes" {
			return "declined", nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("mkdir ~/.claude: %w", err)
	}
	if existed {
		if _, err := install.BackupFile(path); err != nil {
			return "", fmt.Errorf("backup: %w", err)
		}
	}
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	if strings.Contains(content, domainBlockStart) {
		return "updated", nil
	}
	return "installed", nil
}
