package acp

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
)

type modelInfo struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Cost       struct {
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
	} `json:"cost"`
}

// discoverFreeModels corre `opencode models --verbose` y devuelve los refs
// provider/id de los modelos con costo 0.
func discoverFreeModels(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "opencode", "models", "--verbose").Output()
	if err != nil {
		return nil, err
	}
	return parseFreeModels(out), nil
}

// parseFreeModels extrae los objetos JSON top-level del output y se queda con
// los de cost.input==0 && cost.output==0.
func parseFreeModels(out []byte) []string {
	var free []string
	for _, block := range topLevelJSONBlocks(out) {
		var m modelInfo
		if json.Unmarshal(block, &m) != nil {
			continue
		}
		if m.ID == "" || m.ProviderID == "" {
			continue
		}
		if m.Cost.Input == 0 && m.Cost.Output == 0 {
			free = append(free, m.ProviderID+"/"+m.ID)
		}
	}
	return free
}

// topLevelJSONBlocks separa los bloques {..} de columna 0 del output verbose,
// ignorando las lineas header `provider/id` intercaladas.
func topLevelJSONBlocks(out []byte) [][]byte {
	var blocks [][]byte
	var cur []string
	depth := 0
	for _, line := range strings.Split(string(out), "\n") {
		if depth == 0 {
			if line == "{" {
				depth = 1
				cur = []string{line}
			}
			continue
		}
		cur = append(cur, line)
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth == 0 {
			blocks = append(blocks, []byte(strings.Join(cur, "\n")))
			cur = nil
		}
	}
	return blocks
}
