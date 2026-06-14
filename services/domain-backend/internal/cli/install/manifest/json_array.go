package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type JSONArrayReverser struct{}

func (r *JSONArrayReverser) CanRevert(entry Entry) bool {
	return entry.RevertStrategy == "remove_array_entry"
}

func (r *JSONArrayReverser) Revert(entry Entry) error {
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	jsonKey, _ := entry.RevertMetadata["json_key"].(string)
	matchField, _ := entry.RevertMetadata["match_field"].(string)
	matchPrefix, _ := entry.RevertMetadata["match_prefix"].(string)

	if jsonKey == "" || matchField == "" || matchPrefix == "" {
		return fmt.Errorf("missing revert metadata")
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}

	keys := strings.Split(jsonKey, ".")
	current := doc
	for i, k := range keys {
		if i == len(keys)-1 {
			arr, ok := current[k].([]any)
			if !ok {
				return fmt.Errorf("key %s is not an array", jsonKey)
			}
			var newArr []any
			for _, item := range arr {
				m, ok := item.(map[string]any)
				if ok {
					if v, exists := m[matchField]; exists {
						if s, ok := v.(string); ok && strings.HasPrefix(s, matchPrefix) {
							continue
						}
					}
				}
				newArr = append(newArr, item)
			}
			current[k] = newArr
		} else {
			next, ok := current[k].(map[string]any)
			if !ok {
				return fmt.Errorf("key %s is not an object", k)
			}
			current = next
		}
	}

	newData, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	return os.WriteFile(entry.Path, newData, 0o600)
}
