package claudehook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nunezlagos/domain/internal/cli/install"
)

// HasDomainHook indica si existe CUALQUIER hook domain (viejo o actual).
func HasDomainHook(doc map[string]any) bool {
	for _, h := range getSessionStart(doc) {
		typ, _ := h["type"].(string)
		cmd, _ := h["command"].(string)
		if typ == "command" && strings.HasPrefix(strings.TrimSpace(cmd), "domain setup auto-detect") {
			return true
		}
	}
	return false
}

// DomainHookUpToDate indica si el hook domain existe Y tiene el comando ACTUAL
// (con --session-context). Si está pero desactualizado → false (toca upgrade).
func DomainHookUpToDate(doc map[string]any) bool {
	for _, h := range getSessionStart(doc) {
		typ, _ := h["type"].(string)
		cmd, _ := h["command"].(string)
		if typ == "command" && strings.TrimSpace(cmd) == domainHookCommand {
			return true
		}
	}
	return false
}

// AddDomainHook hace UPSERT: si ya hay un hook domain (viejo), reemplaza su
// command por el actual; si no hay ninguno, lo agrega. No duplica.
func AddDomainHook(doc map[string]any) map[string]any {
	result := make(map[string]any, len(doc)+1)
	for k, v := range doc {
		result[k] = v
	}

	hooks, _ := result["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		result["hooks"] = hooks
	}

	ss, _ := hooks["SessionStart"].([]any)
	newSS := make([]any, 0, len(ss)+1)
	upgraded := false
	for _, item := range ss {
		h, ok := item.(map[string]any)
		if ok {
			typ, _ := h["type"].(string)
			cmd, _ := h["command"].(string)
			if typ == "command" && strings.HasPrefix(strings.TrimSpace(cmd), "domain setup auto-detect") {
				h["command"] = domainHookCommand // upgrade in-place
				upgraded = true
			}
		}
		newSS = append(newSS, item)
	}
	if !upgraded {
		newSS = append(newSS, map[string]any{
			"type":    "command",
			"command": domainHookCommand,
		})
	}
	hooks["SessionStart"] = newSS

	return result
}

func InstallClaudeHook(nonInteractive bool, autoAccept bool) (string, error) {
	path := SettingsPath()

	doc, raw, err := ReadSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}

	if DomainHookUpToDate(doc) {
		return "already_installed", nil
	}
	wasUpgrade := HasDomainHook(doc) // existe pero desactualizado → upgrade

	if nonInteractive && !autoAccept {
		return "skipped", nil
	}

	newDoc := AddDomainHook(doc)
	newRaw, err := json.MarshalIndent(newDoc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal settings: %w", err)
	}

	if !autoAccept {
		fmt.Println("Diff for ~/.claude/settings.json:")
		if raw != nil {
			fmt.Printf("  before: %s\n", string(raw))
		} else {
			fmt.Println("  before: (file does not exist)")
		}
		fmt.Printf("  after:  %s\n", string(newRaw))
		fmt.Print("Apply? [y/N] ")
		var response string
		fmt.Scanln(&response)
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return "declined", nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("mkdir ~/.claude: %w", err)
	}

	if raw != nil {
		if _, err := install.BackupFile(path); err != nil {
			return "", fmt.Errorf("backup: %w", err)
		}
	}

	if err := os.WriteFile(path, newRaw, 0o600); err != nil {
		return "", fmt.Errorf("write settings: %w", err)
	}

	if wasUpgrade {
		return "updated", nil
	}
	return "installed", nil
}
