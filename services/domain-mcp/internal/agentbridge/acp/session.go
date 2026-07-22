// Package acp implementa el lado cliente del Agent Client Protocol: domain lanza
// y maneja un agente (opencode acp) por stdio para usarlo como cerebro server-side.
// El SDK de comunidad github.com/coder/acp-go-sdk queda aislado tras esta API.
package acp

import (
	"context"
	"fmt"
	"io"

	acpsdk "github.com/coder/acp-go-sdk"
)

// Session es un cliente ACP sobre un transporte ya conectado. El transporte es
// inyectable (io) para poder testear con net.Pipe sin subproceso; el spawn del
// proceso real vive en process.go
type Session struct {
	conn *acpsdk.ClientSideConnection
	h    *handler
	cwd  string
	// mcp es el server MCP que se le anuncia al agente; nil = núcleo liviano
	// (sin tools, sesión aislada como en DOMAINSERV-63)
	mcp *acpsdk.McpServer
}

// newSession cablea un cliente ACP mínimo (sin workspace ni MCP) sobre un
// transporte inyectable: peerIn es el stdin del agente (domain escribe) y
// peerOut su stdout (domain lee). Es el núcleo liviano usado por el provider
// compartido y por los tests con net.Pipe.
func newSession(peerIn io.Writer, peerOut io.Reader, cwd string) *Session {
	return newSessionWithHandler(peerIn, peerOut, cwd, &handler{}, nil)
}

// newSessionWithHandler permite inyectar un handler ya configurado (workspace +
// permissionMode) y un McpServer para el path nativo seguro (DOMAINSERV-85).
func newSessionWithHandler(peerIn io.Writer, peerOut io.Reader, cwd string, h *handler, mcp *acpsdk.McpServer) *Session {
	return &Session{
		conn: acpsdk.NewClientSideConnection(h, peerIn, peerOut),
		h:    h,
		cwd:  cwd,
		mcp:  mcp,
	}
}

// Prompt corre un turno one-shot (initialize → session/new → session/prompt) y
// devuelve el texto acumulado del agente cuando el turno termina. Si la sesión
// declara un McpServer, exige que el agente anuncie capability http (gating).
func (s *Session) Prompt(ctx context.Context, text string) (string, error) {
	initResp, err := s.conn.Initialize(ctx, acpsdk.InitializeRequest{ProtocolVersion: acpsdk.ProtocolVersionNumber})
	if err != nil {
		return "", fmt.Errorf("acp initialize: %w", err)
	}
	mcpServers, err := s.mcpServers(initResp)
	if err != nil {
		return "", err
	}
	sess, err := s.conn.NewSession(ctx, acpsdk.NewSessionRequest{Cwd: s.cwd, McpServers: mcpServers})
	if err != nil {
		return "", fmt.Errorf("acp new session: %w", err)
	}
	if _, err := s.conn.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: sess.SessionId,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(text)},
	}); err != nil {
		return "", fmt.Errorf("acp prompt: %w", err)
	}
	return s.h.take(), nil
}

// mcpServers decide el slice de McpServer del session/new. Sin mcp configurado
// devuelve vacío (núcleo liviano). Con mcp, exige capability http del agente:
// sin ella no arma una sesión que quedaría sin tools.
func (s *Session) mcpServers(initResp acpsdk.InitializeResponse) ([]acpsdk.McpServer, error) {
	if s.mcp == nil {
		return []acpsdk.McpServer{}, nil
	}
	if !initResp.AgentCapabilities.McpCapabilities.Http {
		return nil, fmt.Errorf("acp: el agente no anuncia MCP http; no se arma sesión nativa sin tools")
	}
	return []acpsdk.McpServer{*s.mcp}, nil
}
