package mcpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	mcp "github.com/mark3labs/mcp-go/mcp"
)

// buildCacheKey: orgID + ":" + toolName + ":" + sha256(args ordenadas).
func buildCacheKey(orgID, toolName string, req mcp.CallToolRequest) string {
	args := req.GetArguments()
	canon := canonicalJSON(args)
	sum := sha256.Sum256([]byte(canon))
	return orgID + ":" + toolName + ":" + hex.EncodeToString(sum[:16])
}

// canonicalJSON: re-serializa con keys ordenadas. Necesario para que
// `{"a":1,"b":2}` y `{"b":2,"a":1}` produzcan el mismo hash.
func canonicalJSON(v any) string {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := "{"
		for i, k := range keys {
			if i > 0 {
				out += ","
			}
			out += `"` + k + `":` + canonicalJSON(x[k])
		}
		return out + "}"
	case []any:
		out := "["
		for i, e := range x {
			if i > 0 {
				out += ","
			}
			out += canonicalJSON(e)
		}
		return out + "]"
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func encodeCachedResult(r *mcp.CallToolResult) ([]byte, error) {
	if r == nil {
		return []byte("null"), nil
	}
	return json.Marshal(r)
}

func decodeCachedResult(b []byte) *mcp.CallToolResult {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var r mcp.CallToolResult
	if err := json.Unmarshal(b, &r); err != nil {

		return nil
	}
	return &r
}
