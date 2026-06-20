package propagate

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type propagatePathsConfig struct {
	Paths []string `json:"paths"`
}

func LoadPropagatePaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(home, ".config", "domain", "propagate-paths.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{"~/Proyectos"}, nil
		}
		return nil, err
	}

	var cfg propagatePathsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return []string{"~/Proyectos"}, nil
	}

	if len(cfg.Paths) == 0 {
		return []string{"~/Proyectos"}, nil
	}

	return cfg.Paths, nil
}
