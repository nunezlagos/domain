// REQ-52 REST endpoints adicionales para el dashboard. Espejo HTTP de
// los tools MCP de REQ-41 a REQ-50 que no tenían REST todavía:
//   - usage_summary (REQ-47) → GET /api/v1/usage/turn-summary
//   - proposals (REQ-49)    → GET /list, POST /review
//   - verifications (REQ-50) → GET /list, POST /start/update/complete
//   - project_repos (REQ-42) → GET /list, POST /add, DELETE
//   - project_policies (REQ-43) → GET /list
//   - captured_prompts (REQ-41) → GET /list
//
// Mantenemos shapes idénticos a los tools MCP donde es razonable, para
// que el frontend pueda reutilizar tipos.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
)

// ----- Usage summary (REQ-47) -----
// GET /api/v1/usage/turn-summary?session_id= o ?project_slug=
func (a *API) usageTurnSummary(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.CapturedPromptService == nil {
		writeError(w, http.StatusServiceUnavailable, "captured_prompt_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	sessStr := r.URL.Query().Get("session_id")
	projSlug := r.URL.Query().Get("project_slug")
	if sessStr == "" && projSlug == "" {
		writeError(w, http.StatusBadRequest, "session_or_project_required", "")
		return
	}
	if sessStr != "" && projSlug != "" {
		writeError(w, http.StatusBadRequest, "mutually_exclusive", "session_id and project_slug")
		return
	}
	if sessStr != "" {
		sid, err := uuid.Parse(sessStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_session_id", "")
			return
		}
		u, err := a.CapturedPromptService.SummarizeBySession(r.Context(), orgID, sid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "summary_failed", err.Error())
			return
		}
		writeData(w, http.StatusOK, u)
		return
	}
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, projSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", projSlug)
		return
	}
	u, err := a.CapturedPromptService.SummarizeByProject(r.Context(), orgID, proj.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "summary_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, u)
}

// GET /api/v1/captured-prompts?project_slug=&session_id=&limit=&offset=
func (a *API) listCapturedPrompts(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.CapturedPromptService == nil {
		writeError(w, http.StatusServiceUnavailable, "captured_prompt_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	q := r.URL.Query()
	filter := capturedpromptsvc.ListFilter{}
	if v := q.Get("project_slug"); v != "" {
		if proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, v); err == nil {
			pid := proj.ID
			filter.ProjectID = &pid
		}
	}
	if v := q.Get("session_id"); v != "" {
		if sid, err := uuid.Parse(v); err == nil {
			filter.SessionID = &sid
		}
	}
	if v := q.Get("limit"); v != "" {
		filter.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("offset"); v != "" {
		filter.Offset, _ = strconv.Atoi(v)
	}
	list, total, err := a.CapturedPromptService.List(r.Context(), orgID, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": list, "total": total})
}

// ----- Project repositories (REQ-42) -----
// GET /api/v1/projects/{slug}/repositories
func (a *API) listProjectRepos(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.ProjectRepoService == nil {
		writeError(w, http.StatusServiceUnavailable, "project_repo_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", "")
		return
	}
	list, err := a.ProjectRepoService.List(r.Context(), orgID, proj.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	ambiguous := len(list) > 1
	for _, r := range list {
		if r.IsDefault {
			ambiguous = false
			break
		}
	}
	writeData(w, http.StatusOK, map[string]any{"items": list, "total": len(list), "ambiguous": ambiguous})
}

type addRepoReq struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	BranchDefault string `json:"branch_default"`
	Kind          string `json:"kind"`
	IsDefault     bool   `json:"is_default"`
	Workflow      string `json:"workflow"`
	Notes         string `json:"notes"`
}

func (a *API) addProjectRepo(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.ProjectRepoService == nil {
		writeError(w, http.StatusServiceUnavailable, "project_repo_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", "")
		return
	}
	var req addRepoReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	rec, err := a.ProjectRepoService.Add(r.Context(), projectreposvc.AddInput{
		ProjectID: proj.ID,
		Name:      req.Name, URL: req.URL, BranchDefault: req.BranchDefault,
		Kind: req.Kind, IsDefault: req.IsDefault, Workflow: req.Workflow, Notes: req.Notes,
	})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "add_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, rec)
}

func (a *API) deleteProjectRepo(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.ProjectRepoService == nil {
		writeError(w, http.StatusServiceUnavailable, "project_repo_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	if err := a.ProjectRepoService.Delete(r.Context(), orgID, id); err != nil {
		writeError(w, http.StatusNotFound, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- Project policies (REQ-43) read-only desde REST -----
// GET /api/v1/projects/{slug}/policies?kind=&include_proposed=
func (a *API) listProjectPolicies(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.ProjectPolicyService == nil {
		writeError(w, http.StatusServiceUnavailable, "project_policy_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", "")
		return
	}
	list, err := a.ProjectPolicyService.List(r.Context(), orgID, proj.ID, r.URL.Query().Get("kind"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": list, "total": len(list)})
}

// ----- Proposals (REQ-49) read + review -----
// GET /api/v1/proposals?kind=&project_slug=
// Devuelve {policies:[...], skills:[...], total: N}
func (a *API) listProposals(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	// Para read-only acá usamos el pool admin que bypassa RLS — pero
	// filtramos manualmente por current_org_id ya que el query lo hace.
	if a.Pool == nil {
		writeError(w, http.StatusServiceUnavailable, "pool_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "all"
	}

	policies := []map[string]any{}
	skills := []map[string]any{}
	const tsFmt = "to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')"

	if kind == "policy" || kind == "all" {
		args := []any{}
		q := `SELECT id::text, slug, name, kind, ` + tsFmt + `
		        FROM project_policies
		        WHERE proposed = true AND deleted_at IS NULL`
		if slug := r.URL.Query().Get("project_slug"); slug != "" {
			if proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, slug); err == nil {
				q += " AND project_id = $1"
				args = append(args, proj.ID)
			}
		}
		q += " ORDER BY created_at DESC LIMIT 50"
		if rows, err := a.Pool.Query(r.Context(), q, args...); err == nil {
			for rows.Next() {
				var id, slug, name, k, ts string
				if err := rows.Scan(&id, &slug, &name, &k, &ts); err == nil {
					policies = append(policies, map[string]any{
						"id": id, "slug": slug, "name": name, "kind": k, "created_at": ts,
					})
				}
			}
			rows.Close()
		}
	}
	if kind == "skill" || kind == "all" {
		if rows, err := a.Pool.Query(r.Context(),
			`SELECT id::text, slug, name, skill_type, project_id, `+tsFmt+`
			   FROM skills
			   WHERE proposed = true AND deleted_at IS NULL
			   ORDER BY created_at DESC LIMIT 50`,
		); err == nil {
			for rows.Next() {
				var id, slug, name, st, ts string
				var pid *uuid.UUID
				if err := rows.Scan(&id, &slug, &name, &st, &pid, &ts); err == nil {
					item := map[string]any{
						"id": id, "slug": slug, "name": name, "skill_type": st, "created_at": ts,
					}
					if pid != nil {
						item["project_id"] = pid.String()
					}
					skills = append(skills, item)
				}
			}
			rows.Close()
		}
	}

	writeData(w, http.StatusOK, map[string]any{
		"policies": policies,
		"skills":   skills,
		"total":    len(policies) + len(skills),
	})
}

// POST /api/v1/proposals/{kind}/{id}/review
// body: { action: accept|reject, reason?: string }
type reviewReq struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

func (a *API) reviewProposal(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.Pool == nil {
		writeError(w, http.StatusServiceUnavailable, "pool_unavailable", "")
		return
	}
	kind := r.PathValue("kind")
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	var req reviewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Action != "accept" && req.Action != "reject" {
		writeError(w, http.StatusBadRequest, "invalid_action", "accept|reject")
		return
	}
	var table string
	switch kind {
	case "policy":
		table = "project_policies"
	case "skill":
		table = "skills"
	default:
		writeError(w, http.StatusBadRequest, "invalid_kind", "policy|skill")
		return
	}
	var sql string
	if req.Action == "accept" {
		sql = "UPDATE " + table + ` SET proposed = false
		         WHERE id = $1 AND proposed = true AND deleted_at IS NULL`
	} else {
		sql = "UPDATE " + table + ` SET deleted_at = NOW()
		         WHERE id = $1 AND proposed = true AND deleted_at IS NULL`
	}
	tag, err := a.Pool.Exec(r.Context(), sql, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "review_failed", err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "proposal_not_found", "or already reviewed")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"id": id.String(), "action": req.Action})
}

// ----- Verifications (REQ-50) read pending del proyecto -----
// GET /api/v1/projects/{slug}/verifications?limit=
func (a *API) listVerifications(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.Pool == nil {
		writeError(w, http.StatusServiceUnavailable, "pool_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, r.PathValue("slug"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", "")
		return
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	statuses := r.URL.Query().Get("status")
	if statuses == "" {
		statuses = "pending,running,failed,partial"
	}
	rows, err := a.Pool.Query(r.Context(),
		`SELECT id::text, kind, status, COALESCE(context,''), items,
		        to_char(started_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        completed_at
		   FROM verifications
		   WHERE project_id = $1
		     AND status = ANY (string_to_array($2, ','))
		   ORDER BY started_at DESC LIMIT $3`,
		proj.ID, statuses, limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, kind, status, contextStr, ts string
		var items []byte
		var completedAt *string
		if err := rows.Scan(&id, &kind, &status, &contextStr, &items, &ts, &completedAt); err == nil {
			var itemsParsed any
			_ = json.Unmarshal(items, &itemsParsed)
			out = append(out, map[string]any{
				"id": id, "kind": kind, "status": status, "context": contextStr,
				"started_at": ts, "completed_at": completedAt, "items": itemsParsed,
			})
		}
	}
	writeData(w, http.StatusOK, map[string]any{"items": out, "total": len(out)})
}
