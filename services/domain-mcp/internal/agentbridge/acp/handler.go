package acp

import (
	"context"
	"errors"
	"strings"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
)

// errUnsupported lo devuelven los handlers de fs/terminal cuando la operación no
// está habilitada: sin workspace (núcleo liviano, DOMAINSERV-63) o bajo un
// PermissionMode que la rechaza. fs con workspace vive en handler_fs.go.
var errUnsupported = errors.New("acp: operación de workspace no soportada en esta sesión")

// handler implementa acpsdk.Client. Acumula el texto de los AgentMessageChunk
// del stream session/update. Las operaciones de fs que opencode DELEGA vía ACP
// pasan por el workspace (si hay uno); terminal degrada a error y los permisos
// se rechazan por default. Esto NO aísla al subproceso opencode (mismo uid, sin
// namespace): solo acota las ops delegadas — el aislamiento real es DOMAINSERV-86.
type handler struct {
	mu  sync.Mutex
	buf strings.Builder
	// ws acota los reads/writes al root del run; nil = fs no soportado
	ws *Workspace
	// permissionMode gobierna writes/permisos; "deny-all" (default) los rechaza
	permissionMode string
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
