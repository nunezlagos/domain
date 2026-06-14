package autodetect

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ActionType string

const (
	ActionSymlink        ActionType = "symlink"
	ActionJSONUpsert     ActionType = "json_upsert"
	ActionOpenCodeGen    ActionType = "opencode_generate"
)

type Action struct {
	Type       ActionType `json:"type"`
	Path       string     `json:"path"`
	Target     string     `json:"target,omitempty"`
	Key        string     `json:"key,omitempty"`
	BeforeHash string     `json:"before_hash,omitempty"`
	AfterHash  string     `json:"after_hash,omitempty"`
}

func Apply(projectDir string) ([]Action, error) {
	actions, err := planActions(projectDir)
	if err != nil {
		return nil, err
	}
	var applied []Action
	for _, a := range actions {
		switch a.Type {
		case ActionSymlink:
			linkPath := filepath.Join(projectDir, a.Path)
			if existing, err := os.Readlink(linkPath); err == nil {
				if existing == a.Target {
					continue
				}
				os.Remove(linkPath)
			} else if !os.IsNotExist(err) {
				continue
			}
			if err := os.Symlink(a.Target, linkPath); err != nil {
				return nil, fmt.Errorf("symlink %s -> %s: %w", linkPath, a.Target, err)
			}
			applied = append(applied, a)
		case ActionJSONUpsert:
			mcpPath := filepath.Join(projectDir, a.Path)
			beforeHash, err := fileHash(mcpPath)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			if err := upsertMCPServer(mcpPath); err != nil {
				return nil, fmt.Errorf("upsert .mcp.json: %w", err)
			}
			afterHash, err := fileHash(mcpPath)
			if err != nil {
				return nil, err
			}
			if beforeHash != afterHash {
				a.BeforeHash = beforeHash
				a.AfterHash = afterHash
				applied = append(applied, a)
			}
		case ActionOpenCodeGen:
			ocPath := filepath.Join(projectDir, "opencode.json")
			if _, err := os.Stat(ocPath); err == nil {
				continue
			}
			if err := generateMinimalOpenCode(projectDir); err != nil {
				return nil, fmt.Errorf("generate opencode.json: %w", err)
			}
			applied = append(applied, a)
		}
	}
	if len(applied) > 0 {
		if err := recordManifest(projectDir, applied); err != nil {
			return applied, err
		}
	}
	return applied, nil
}

func ApplyDryRun(projectDir string) ([]Action, error) {
	return planActions(projectDir)
}

func planActions(projectDir string) ([]Action, error) {
	st, err := Detect(projectDir)
	if err != nil {
		return nil, err
	}

	var actions []Action
	switch st {
	case StateClaudeMDOnly:
		if !symlinkExists(projectDir, "AGENTS.md", "CLAUDE.md") {
			actions = append(actions, Action{
				Type:   ActionSymlink,
				Path:   "AGENTS.md",
				Target: "CLAUDE.md",
			})
		}
	case StateMCPJSONOnly:
		if !mcpHasDomain(filepath.Join(projectDir, ".mcp.json")) {
			actions = append(actions, Action{
				Type: ActionJSONUpsert,
				Path: ".mcp.json",
				Key:  "mcpServers.domain",
			})
		}
	case StateOpenCodeConfigOnly:
		actions = append(actions, planOpenCodeActions(projectDir)...)
	case StateAllPresent:
		if claudeMDExists(projectDir) && !agentsMDExists(projectDir) &&
			!symlinkExists(projectDir, "AGENTS.md", "CLAUDE.md") {
			actions = append(actions, Action{
				Type:   ActionSymlink,
				Path:   "AGENTS.md",
				Target: "CLAUDE.md",
			})
		}
		if fileExists(projectDir, ".mcp.json") && !mcpHasDomain(filepath.Join(projectDir, ".mcp.json")) {
			actions = append(actions, Action{
				Type: ActionJSONUpsert,
				Path: ".mcp.json",
				Key:  "mcpServers.domain",
			})
		}
		if fileExists(projectDir, "opencode.json") && !openCodeHasDomain(filepath.Join(projectDir, "opencode.json")) {
			actions = append(actions, planOpenCodeActions(projectDir)...)
		}
	case StateNone:
		actions = append(actions, Action{
			Type: ActionOpenCodeGen,
			Path: "opencode.json",
		})
	}
	return actions, nil
}

func planOpenCodeActions(projectDir string) []Action {
	if openCodeHasDomain(filepath.Join(projectDir, "opencode.json")) {
		return nil
	}
	return []Action{
		{
			Type: ActionOpenCodeGen,
			Path: "opencode.json",
		},
	}
}

func symlinkExists(dir, link, target string) bool {
	linkPath := filepath.Join(dir, link)
	existing, err := os.Readlink(linkPath)
	return err == nil && existing == target
}

func fileExists(dir string, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func claudeMDExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "CLAUDE.md"))
	return err == nil
}

func agentsMDExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "AGENTS.md"))
	return err == nil
}

func openCodeHasDomain(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return false
	}
	mcp, _ := doc["mcp"].(map[string]any)
	_, ok := mcp["domain"]
	return ok
}

func mcpHasDomain(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return false
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	_, ok := servers["domain"]
	return ok
}

func upsertMCPServer(path string) error {
	var doc map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			doc = map[string]any{}
		} else {
			return err
		}
	} else {
		if err := json.Unmarshal(data, &doc); err != nil {
			return err
		}
	}

	servers, _ := doc["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, ok := servers["domain"]; ok {
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	mcpBinary := findDomainMCP(exe)

	servers["domain"] = map[string]any{
		"command": mcpBinary,
		"args":    []any{},
	}
	doc["mcpServers"] = servers

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

func findDomainMCP(exePath string) string {
	dir := filepath.Dir(exePath)
	candidate := filepath.Join(dir, "domain-mcp")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return "domain-mcp"
}

func generateMinimalOpenCode(projectDir string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	mcpBinary := findDomainMCP(exe)

	doc := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"domain": map[string]any{
				"type":    "local",
				"command": []any{mcpBinary},
				"enabled": true,
			},
		},
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectDir, "opencode.json"), out, 0o600)
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}

// recordManifest records actions to the local manifest.
func recordManifest(projectDir string, actions []Action) error {
	manifestDir := filepath.Join(projectDir, ".domain")
	manifestPath := filepath.Join(manifestDir, "install-manifest.json")

	// Try to write to project-local dir first
	err := os.MkdirAll(manifestDir, 0o755)
	if err != nil {
		manifestPath = fallbackManifestPath(projectDir)
	}

	existing, err := ReadManifest(manifestPath)
	if err != nil {
		existing = &Manifest{
			Version:       1,
			DomainVersion: "dev",
			AppliedAt:     time.Now().UTC(),
		}
	}
	existing.Actions = append(existing.Actions, actions...)
	existing.AppliedAt = time.Now().UTC()

	return WriteManifest(manifestPath, existing)
}

func fallbackManifestPath(projectDir string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	fallbackDir := filepath.Join(home, ".config", "domain", "orphan-manifests")
	os.MkdirAll(fallbackDir, 0o700)
	basename := filepath.Base(projectDir)
	return filepath.Join(fallbackDir, basename+".json")
}


