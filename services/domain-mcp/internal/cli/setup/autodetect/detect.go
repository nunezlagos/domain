package autodetect

import (
	"fmt"
	"os"
	"path/filepath"
)

type State int

const (
	StateNone State = iota
	StateClaudeMDOnly
	StateMCPJSONOnly
	StateOpenCodeConfigOnly
	StateAllPresent
)

var detectionPaths = []string{
	".claude",
	".opencode",
	".cursor",
	".mcp.json",
	"AGENTS.md",
	"CLAUDE.md",
	"opencode.json",
}

// ruleFilePaths son los archivos/dirs de reglas de IA que un repo puede traer.
// Su presencia es relevante para la precedencia: domain manda sobre ellos en
// memoria/skills/SDD (ver agentprotocol.Stub).
var ruleFilePaths = []string{
	"AGENTS.md",
	"CLAUDE.md",
	"CLAUDE.local.md",
	".claude",
	".cursorrules",
	".cursor",
	".windsurf",
	".github/copilot-instructions.md",
	".aider.conf.yml",
	".clinerules",
}

// DetectRuleFiles devuelve los paths (relativos a dir) de archivos de reglas
// de IA presentes en el repo. Se usa para informar al cliente qué reglas
// locales quedan subordinadas a domain.
func DetectRuleFiles(dir string) []string {
	var found []string
	for _, p := range ruleFilePaths {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			found = append(found, p)
		}
	}
	return found
}

func Detect(path string) (State, error) {
	info, err := os.Stat(path)
	if err != nil {
		return StateNone, fmt.Errorf("path %s is not accessible: %w", path, err)
	}
	if !info.IsDir() {
		return StateNone, fmt.Errorf("path %s is not a directory", path)
	}

	hasClaudeMD := false
	hasMCPJSON := false
	hasOpenCodeConfig := false
	hasOthers := false

	for _, p := range detectionPaths {
		fullPath := filepath.Join(path, p)
		if _, err := os.Stat(fullPath); err == nil {
			switch p {
			case "CLAUDE.md":
				hasClaudeMD = true
			case ".mcp.json":
				hasMCPJSON = true
			case "opencode.json":
				hasOpenCodeConfig = true
			default:
				hasOthers = true
			}
		}
	}

	if hasClaudeMD && !hasMCPJSON && !hasOpenCodeConfig && !hasOthers {
		return StateClaudeMDOnly, nil
	}
	if hasMCPJSON && !hasClaudeMD && !hasOpenCodeConfig && !hasOthers {
		return StateMCPJSONOnly, nil
	}
	if hasOpenCodeConfig && !hasClaudeMD && !hasMCPJSON && !hasOthers {
		return StateOpenCodeConfigOnly, nil
	}
	if !hasClaudeMD && !hasMCPJSON && !hasOpenCodeConfig && !hasOthers {
		return StateNone, nil
	}
	return StateAllPresent, nil
}
