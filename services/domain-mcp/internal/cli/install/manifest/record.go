package manifest

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const currentInstallID = "in-memory-current"

func Record(path string, action Action) (Entry, error) {
	m, err := ReadManifest(path)
	if err != nil {
		return Entry{}, fmt.Errorf("read manifest: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	entry := Entry{
		ID:              uuid.New().String(),
		Type:            action.Type,
		Path:            action.Path,
		BeforeHash:      action.BeforeHash,
		AfterHash:       action.AfterHash,
		Issue:           action.Issue,
		Revertible:      action.Revertible,
		RevertStrategy:  action.RevertStrategy,
		RevertMetadata:  action.RevertMetadata,
		Timestamp:       now,
	}


	var currentInstall *Install
	if len(m.Installs) > 0 {
		last := &m.Installs[len(m.Installs)-1]
		if last.FinishedAt == "" {
			currentInstall = last
		}
	}

	if currentInstall == nil {
		m.Installs = append(m.Installs, Install{
			InstallID: uuid.New().String(),
			StartedAt: now,
		})
		currentInstall = &m.Installs[len(m.Installs)-1]
	}

	currentInstall.Entries = append(currentInstall.Entries, entry)

	if err := WriteManifest(path, m); err != nil {
		return Entry{}, fmt.Errorf("write manifest: %w", err)
	}

	return entry, nil
}
