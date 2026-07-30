package mcpserver

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func toolResultJSON(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal: %v", err)), nil
	}
	// mcp-result-lint:allow este ES el helper canónico que centraliza la construcción
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(b)}},
	}, nil
}
