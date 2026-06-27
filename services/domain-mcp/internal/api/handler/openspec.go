package handler

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	openspecsvc "nunezlagos/domain/internal/service/openspec"
)

// Round-trip openspec por HTTP (CLI `domain openspec export/status/apply`).
//
// Superficie REST (Bearer auth, NO en AuthAllowlist):
//   - GET  /api/v1/openspec/export?project_slug=X&scope=active  DB→repo
//   - POST /api/v1/openspec/status  {files:[{path,content}]}    drift repo↔DB
//   - POST /api/v1/openspec/apply   {files:[...], force:bool}   repo→DB
//
// Reusa el MISMO Engine que los tools MCP (internal/service/openspec): cero
// duplicación de lógica. El server nunca toca el filesystem; export devuelve
// {path: contenido} y el CLI los escribe a disco, apply recibe los .md editados.
//
// Single-tenant (regla dura 1): el orgID sale del Principal (request-time) solo
// para resolver el project por slug; no hay organization_id en openspec.

type openspecFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type openspecFilesReq struct {
	Files []openspecFile `json:"files"`
	Force bool           `json:"force,omitempty"`
}

// openspecExport maneja GET /api/v1/openspec/export.
func (a *API) openspecExport(w http.ResponseWriter, r *http.Request) {
	if a.Openspec == nil || a.Projects == nil {
		writeError(w, http.StatusServiceUnavailable, "openspec_disabled", "")
		return
	}
	slug := strings.TrimSpace(r.URL.Query().Get("project_slug"))
	if slug == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "project_slug requerido")
		return
	}
	orgID, ok := openspecOrgID(w, r)
	if !ok {
		return
	}
	proj, err := a.Projects.GetBySlug(r.Context(), orgID, slug)
	if err != nil || proj == nil {
		writeError(w, http.StatusNotFound, "project_not_found", "project '"+slug+"' no encontrado")
		return
	}
	scope := r.URL.Query().Get("scope")
	changes, err := a.Openspec.Export(r.Context(), proj.ID, scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"project_slug": slug,
		"change_count": len(changes),
		"changes":      changes,
	})
}

// openspecStatus maneja POST /api/v1/openspec/status.
func (a *API) openspecStatus(w http.ResponseWriter, r *http.Request) {
	if a.Openspec == nil {
		writeError(w, http.StatusServiceUnavailable, "openspec_disabled", "")
		return
	}
	var in openspecFilesReq
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "body invalido")
		return
	}
	if len(in.Files) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "files requerido (no vacío)")
		return
	}
	results := a.Openspec.Status(r.Context(), toEngineFiles(in.Files))
	writeData(w, http.StatusOK, map[string]any{
		"change_count": len(results),
		"changes":      results,
	})
}

// openspecApply maneja POST /api/v1/openspec/apply.
func (a *API) openspecApply(w http.ResponseWriter, r *http.Request) {
	if a.Openspec == nil {
		writeError(w, http.StatusServiceUnavailable, "openspec_disabled", "")
		return
	}
	var in openspecFilesReq
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "body invalido")
		return
	}
	if len(in.Files) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "files requerido (no vacío)")
		return
	}
	results := a.Openspec.Apply(r.Context(), toEngineFiles(in.Files), in.Force, openspecActor(r))
	writeData(w, http.StatusOK, map[string]any{
		"change_count": len(results),
		"changes":      results,
	})
}

func toEngineFiles(files []openspecFile) []openspecsvc.File {
	out := make([]openspecsvc.File, 0, len(files))
	for _, f := range files {
		out = append(out, openspecsvc.File{Path: f.Path, Content: f.Content})
	}
	return out
}

// openspecOrgID resuelve el orgID del Principal para acotar el lookup de
// project. Single-org: el orgID viene del session/apikey resolver.
func openspecOrgID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	p, ok := principal(r)
	if !ok || p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return uuid.Nil, false
	}
	orgID, err := uuid.Parse(p.OrganizationID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid org id")
		return uuid.Nil, false
	}
	return orgID, true
}

// openspecActor resuelve el user id del Principal para marcar tasks
// completadas (apply). Si no hay user válido cae al sentinel del Engine.
func openspecActor(r *http.Request) string {
	p, ok := principal(r)
	if !ok || p == nil {
		return "openspec-sync"
	}
	return p.UserID
}
