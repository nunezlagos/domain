package autodetect

import (
	"fmt"
	"os"
	"path/filepath"
)

type State int

const (
	StateNone             State = iota
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
