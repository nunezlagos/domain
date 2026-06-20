package manifest

import "fmt"

type UninstallResult struct {
	Reverted int
	Skipped  int
	Errors   []string
}

func Uninstall(manifestPath string, confirmed bool, dryRun bool) (*UninstallResult, error) {
	if !confirmed {
		return nil, fmt.Errorf("confirm required")
	}

	m, err := ReadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	if dryRun {
		var total int
		for _, inst := range m.Installs {
			total += len(inst.Entries)
		}
		return &UninstallResult{Reverted: total}, nil
	}

	reg := NewReverserRegistry()
	reg.Register("rcfile_append", &BlockMarkerReverser{})
	reg.Register("claude_settings_merge", &JSONArrayReverser{})
	reg.Register("file_create", &FileDeleteReverser{})
	reg.Register("symlink", &FileDeleteReverser{})
	reg.Register("file_modify", &FileDeleteReverser{})

	result := &UninstallResult{}

	// Process in reverse order
	for i := len(m.Installs) - 1; i >= 0; i-- {
		inst := &m.Installs[i]
		var remainingEntries []Entry
		for j := len(inst.Entries) - 1; j >= 0; j-- {
			entry := inst.Entries[j]

			if !entry.Revertible {
				remainingEntries = append(remainingEntries, entry)
				continue
			}

			// Hash check
			if entry.AfterHash != "" {
				currentHash, err := HashFile(entry.Path)
				if err != nil || currentHash != entry.AfterHash {
					result.Skipped++
					result.Errors = append(result.Errors, fmt.Sprintf("%s: modified externally; skipping revert", entry.Path))
					remainingEntries = append(remainingEntries, entry)
					continue
				}
			}

			rev, ok := reg.Get(entry.Type)
			if !ok {
				result.Skipped++
				result.Errors = append(result.Errors, fmt.Sprintf("%s: no reverser for type %s", entry.Path, entry.Type))
				remainingEntries = append(remainingEntries, entry)
				continue
			}

			if !rev.CanRevert(entry) {
				result.Skipped++
				result.Errors = append(result.Errors, fmt.Sprintf("%s: reverser cannot handle entry", entry.Path))
				remainingEntries = append(remainingEntries, entry)
				continue
			}

			if err := rev.Revert(entry); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: revert error: %v", entry.Path, err))
				remainingEntries = append(remainingEntries, entry)
				continue
			}

			result.Reverted++
		}
		inst.Entries = remainingEntries
	}

	// Remove empty installs
	var activeInstalls []Install
	for _, inst := range m.Installs {
		if len(inst.Entries) > 0 {
			activeInstalls = append(activeInstalls, inst)
		}
	}
	m.Installs = activeInstalls

	if err := WriteManifest(manifestPath, m); err != nil {
		return result, fmt.Errorf("write manifest after uninstall: %w", err)
	}

	return result, nil
}
