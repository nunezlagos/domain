package autodetect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Manifest struct {
	Version       int      `json:"version"`
	DomainVersion string   `json:"domain_version"`
	AppliedAt     time.Time `json:"applied_at"`
	Actions       []Action `json:"actions"`
}

func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	return &m, nil
}

func WriteManifest(path string, m *Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir manifest dir: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func AppendToManifest(path string, action Action) error {
	m, err := ReadManifest(path)
	if err != nil {
		m = &Manifest{
			Version:       1,
			DomainVersion: "dev",
			AppliedAt:     time.Now().UTC(),
		}
	}
	m.Actions = append(m.Actions, action)
	m.AppliedAt = time.Now().UTC()
	return WriteManifest(path, m)
}
