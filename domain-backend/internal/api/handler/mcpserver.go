package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/mcpserver"
)

type createMCPServerBody struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport,omitempty"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
}

// POST /api/v1/mcp-servers
func (a *API) createMCPServer(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.MCPServerService == nil {
		writeError(w, http.StatusServiceUnavailable, "mcp_not_configured", "")
		return
	}
	var in createMCPServerBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	srv, err := a.MCPServerService.Create(r.Context(), orgID,
		mcpserver.CreateInput{
			Name: in.Name, Transport: in.Transport, Command: in.Command,
			Args: in.Args, Env: in.Env, URL: in.URL,
		})
	if err != nil {
		switch {
		case errors.Is(err, mcpserver.ErrInvalidTransport):
			writeError(w, http.StatusUnprocessableEntity, "invalid_transport", err.Error())
		case errors.Is(err, mcpserver.ErrCommandRequired):
			writeError(w, http.StatusUnprocessableEntity, "command_required", "")
		default:
			writeError(w, http.StatusInternalServerError, "create", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/mcp-servers/"+srv.ID.String())
	writeData(w, http.StatusCreated, srv)
}

// GET /api/v1/mcp-servers
func (a *API) listMCPServers(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	srvs, err := a.MCPServerService.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, srvs)
}

// GET /api/v1/mcp-servers/{id}
func (a *API) getMCPServer(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	srv, err := a.MCPServerService.Get(r.Context(), orgID, id)
	if errors.Is(err, mcpserver.ErrUnknown) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, srv)
}

// DELETE /api/v1/mcp-servers/{id}
func (a *API) deleteMCPServer(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	if err := a.MCPServerService.Delete(r.Context(), orgID, id); err != nil {
		if errors.Is(err, mcpserver.ErrUnknown) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/mcp-servers/{id}/sync-tools
func (a *API) syncMCPTools(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	tools, err := a.MCPServerService.SyncTools(r.Context(), orgID, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "mcp_sync_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, tools)
}

// GET /api/v1/mcp-servers/{id}/tools
func (a *API) listMCPTools(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	tools, err := a.MCPServerService.ListTools(r.Context(), orgID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_tools", err.Error())
		return
	}
	writeData(w, http.StatusOK, tools)
}

type invokeMCPToolBody struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
}

// POST /api/v1/mcp-servers/{id}/invoke
func (a *API) invokeMCPTool(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	var b invokeMCPToolBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if b.ToolName == "" {
		writeError(w, http.StatusUnprocessableEntity, "tool_name_required", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	res, err := a.MCPServerService.InvokeTool(r.Context(), orgID, id, b.ToolName, b.Arguments)
	if err != nil {
		writeError(w, http.StatusBadGateway, "mcp_invoke_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, res)
}
