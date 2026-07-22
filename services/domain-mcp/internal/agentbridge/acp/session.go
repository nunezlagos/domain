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
}

// newSession cablea el cliente ACP sobre un transporte inyectable: peerIn es el
// stdin del agente (domain escribe) y peerOut su stdout (domain lee)
func newSession(peerIn io.Writer, peerOut io.Reader, cwd string) *Session {
	h := &handler{}
	return &Session{
		conn: acpsdk.NewClientSideConnection(h, peerIn, peerOut),
		h:    h,
		cwd:  cwd,
	}
}

// Prompt corre un turno one-shot (initialize → session/new → session/prompt) y
// devuelve el texto acumulado del agente cuando el turno termina
func (s *Session) Prompt(ctx context.Context, text string) (string, error) {
	if _, err := s.conn.Initialize(ctx, acpsdk.InitializeRequest{ProtocolVersion: acpsdk.ProtocolVersionNumber}); err != nil {
		return "", fmt.Errorf("acp initialize: %w", err)
	}
	sess, err := s.conn.NewSession(ctx, acpsdk.NewSessionRequest{Cwd: s.cwd, McpServers: []acpsdk.McpServer{}})
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
