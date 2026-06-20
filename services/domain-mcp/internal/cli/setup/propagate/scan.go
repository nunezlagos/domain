package propagate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

var DetectableIAConfigs = []string{
	"opencode.json",
	".mcp.json",
	".claude",
	".opencode",
	".cursor",
	"AGENTS.md",
	"CLAUDE.md",
}

type ProjectInfo struct {
	Name             string
	Path             string
	HasDomain        bool
	DomainManifestAt string
	IAConfigs        []string
}

func Scan(rootPath string) ([]ProjectInfo, error) {
	fi, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("scan path %s: %w", rootPath, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("scan path %s is not a directory", rootPath)
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", rootPath, err)
	}

	var infos []ProjectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projPath := filepath.Join(rootPath, entry.Name())
		info := ProjectInfo{
			Name: entry.Name(),
			Path: projPath,
		}

		manifestPath := filepath.Join(projPath, ".domain", "install-manifest.json")
		if _, err := os.Stat(manifestPath); err == nil {
			info.HasDomain = true
			info.DomainManifestAt = manifestPath
		}

		for _, cfg := range DetectableIAConfigs {
			cfgPath := filepath.Join(projPath, cfg)
			if _, err := os.Stat(cfgPath); err == nil {
				info.IAConfigs = append(info.IAConfigs, cfg)
			}
		}

		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos, nil
}
