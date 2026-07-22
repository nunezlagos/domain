package acp

import (
	"context"
	"os"

	acpsdk "github.com/coder/acp-go-sdk"
)

// ReadTextFile acota al workspace root SOLO las lecturas de fs que opencode
// DELEGA vía ACP (resolve rechaza traversal/symlink-escape). NO es un sandbox
// del subproceso: opencode corre con el mismo uid y puede leer el fs del server
// por su cuenta — el aislamiento real del proceso es DOMAINSERV-86. Sin
// workspace (núcleo liviano) la op delegada degrada a errUnsupported.
func (h *handler) ReadTextFile(_ context.Context, req acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	if h.ws == nil {
		return acpsdk.ReadTextFileResponse{}, errUnsupported
	}
	resolved, err := h.ws.resolve(req.Path)
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}
	return acpsdk.ReadTextFileResponse{Content: string(data)}, nil
}

// WriteTextFile es opt-in por PermissionMode; el default deny-all lo rechaza.
// Habilitado, acota al workspace root las escrituras que opencode DELEGA vía
// ACP (no impide que el subproceso escriba fs por fuera de ACP; ver DOMAINSERV-86).
func (h *handler) WriteTextFile(_ context.Context, req acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	if h.ws == nil || h.permissionMode == PermissionDenyAll || h.permissionMode == "" {
		return acpsdk.WriteTextFileResponse{}, errUnsupported
	}
	resolved, err := h.ws.resolve(req.Path)
	if err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	if err := os.WriteFile(resolved, []byte(req.Content), 0o600); err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	return acpsdk.WriteTextFileResponse{}, nil
}
