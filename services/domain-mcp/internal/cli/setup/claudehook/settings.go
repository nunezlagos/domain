package claudehook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const domainHookCommand = `domain setup auto-detect "$PWD" --quiet --session-context`

func SettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func ReadSettings() (map[string]any, []byte, error) {
	path := SettingsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil, nil
		}
		return nil, nil, fmt.Errorf("read settings: %w", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return map[string]any{}, data, nil
	}
	return doc, data, nil
}

func getSessionStart(doc map[string]any) []map[string]any {
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}
	raw, _ := hooks["SessionStart"].([]any)
	if raw == nil {
		return nil
	}
	var result []map[string]any
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}
