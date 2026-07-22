package acp

import acpsdk "github.com/coder/acp-go-sdk"

// buildMcpServer arma el McpServer http-inline que domain le expone al agente:
// las tools reales del MCP autenticadas por bearer + el header de profundidad
// que dispara la barrera anti-reentrancia (excluye agent_run/orchestrate/flow_run).
// El campo Type lo fija el SDK a "http" al serializar.
func buildMcpServer(cfg Config) *acpsdk.McpServer {
	return &acpsdk.McpServer{
		Http: &acpsdk.McpServerHttpInline{
			Name: "domain-mcp",
			Url:  cfg.McpURL,
			Headers: []acpsdk.HttpHeader{
				{Name: "Authorization", Value: "Bearer " + cfg.McpToken},
				{Name: "X-Domain-Agent-Depth", Value: "1"},
			},
		},
	}
}
