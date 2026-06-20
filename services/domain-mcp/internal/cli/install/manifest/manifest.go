package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Manifest struct {
	Version  int       `json:"version"`
	Installs []Install `json:"installs"`
}

type Install struct {
	InstallID  string  `json:"install_id"`
	StartedAt  string  `json:"started_at,omitempty"`
	FinishedAt string  `json:"finished_at,omitempty"`
	Entries    []Entry `json:"entries"`
}

type Entry struct {
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	Path            string         `json:"path"`
	BeforeHash      string         `json:"before_hash,omitempty"`
	AfterHash       string         `json:"after_hash,omitempty"`
	Issue           string         `json:"originating_issue,omitempty"`
	Revertible      bool           `json:"revertible"`
	RevertStrategy  string         `json:"revert_strategy,omitempty"`
	RevertMetadata  map[string]any `json:"revert_metadata,omitempty"`
	Timestamp       string         `json:"timestamp,omitempty"`
}

type Action struct {
	Path            string
	Type            string
	BeforeHash      string
	AfterHash       string
	Issue           string
	Revertible      bool
	RevertStrategy  string
	RevertMetadata  map[string]any
}

func ManifestPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "domain", "install-manifest.json"), nil
}

func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Version: 1}, nil
		}
		return &Manifest{Version: 1}, nil
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return &Manifest{Version: 1}, nil
	}
	if m.Version == 0 {
		m.Version = 1
	}
	return &m, nil
}

func WriteManifest(path string, m *Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir manifest dir: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write tmp manifest: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename manifest: %w", err)
	}
	return nil
}
