package acp

import (
	"context"
	"errors"
	"strings"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
)

// errUnsupported lo devuelven los handlers de fs/terminal: el núcleo liviano
// (DOMAINSERV-63) no soporta operaciones de workspace; la implementación robusta
// es del modo A de agent_run (DOMAINSERV-66)
var errUnsupported = errors.New("acp: operación de workspace no soportada en el núcleo liviano")

// handler implementa acpsdk.Client. Acumula el texto de los AgentMessageChunk
// del stream session/update; fs/terminal degradan a error y los permisos se
// rechazan por default (política segura hasta DOMAINSERV-66)
type handler struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (h *handler) SessionUpdate(_ context.Context, p acpsdk.SessionNotification) error {
	c := p.Update.AgentMessageChunk
	if c == nil || c.Content.Text == nil {
		return nil
	}
	h.mu.Lock()
	h.buf.WriteString(c.Content.Text.Text)
	h.mu.Unlock()
	return nil
}

func (h *handler) take() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.buf.String()
	h.buf.Reset()
	return s
}

func (h *handler) RequestPermission(context.Context, acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	return acpsdk.RequestPermissionResponse{
		Outcome: acpsdk.RequestPermissionOutcome{Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{}},
	}, nil
}

func (h *handler) ReadTextFile(context.Context, acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	return acpsdk.ReadTextFileResponse{}, errUnsupported
}

func (h *handler) WriteTextFile(context.Context, acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	return acpsdk.WriteTextFileResponse{}, errUnsupported
}

func (h *handler) CreateTerminal(context.Context, acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
	return acpsdk.CreateTerminalResponse{}, errUnsupported
}

func (h *handler) KillTerminal(context.Context, acpsdk.KillTerminalRequest) (acpsdk.KillTerminalResponse, error) {
	return acpsdk.KillTerminalResponse{}, errUnsupported
}

func (h *handler) TerminalOutput(context.Context, acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error) {
	return acpsdk.TerminalOutputResponse{}, errUnsupported
}

func (h *handler) ReleaseTerminal(context.Context, acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error) {
	return acpsdk.ReleaseTerminalResponse{}, errUnsupported
}

func (h *handler) WaitForTerminalExit(context.Context, acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error) {
	return acpsdk.WaitForTerminalExitResponse{}, errUnsupported
}
