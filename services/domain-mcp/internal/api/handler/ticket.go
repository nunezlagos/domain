// REQ-52 REST handlers para sistema de tickets (REQ-51). Espejo HTTP
// de los 10 tools MCP domain_ticket_*. Todos requieren auth via apikey
// middleware (que setea principal en ctx) — sin principal, 401.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	ticketsvc "nunezlagos/domain/internal/service/ticket"
)

// POST /api/v1/projects/{slug}/tickets
type createTicketReq struct {
	Title          string    `json:"title"`
	DescriptionMD  string    `json:"description_md"`
	IssueType      string    `json:"issue_type"`
	Priority       string    `json:"priority"`
	ClientSlug     string    `json:"client_slug"`
	AssigneeID     string    `json:"assignee_id"`
	Labels         []string  `json:"labels"`
	ParentID       string    `json:"parent_id"`
	EstimatedHours *float64  `json:"estimated_hours"`
	DueDate        string    `json:"due_date"` // YYYY-MM-DD

	ExternalProvider string `json:"external_provider,omitempty"`
	ExternalID       string `json:"external_id,omitempty"`
	ExternalURL      string `json:"external_url,omitempty"`
}

func (a *API) createTicket(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	slug := r.PathValue("slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "project_slug_required", "")
		return
	}
	var req createTicketReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	userID, _ := uuid.Parse(p.UserID)
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", slug)
		return
	}
	in := ticketsvc.CreateInput{
		OrganizationID: orgID,
		ProjectID:      proj.ID,
		ProjectSlug:    slug,
		Title:          req.Title,
		DescriptionMD:  req.DescriptionMD,
		IssueType:      req.IssueType,
		Priority:       req.Priority,
		ReporterID:     userID,
		Labels:         req.Labels,
		EstimatedHours: req.EstimatedHours,

		ExternalProvider: req.ExternalProvider,
		ExternalID:       req.ExternalID,
		ExternalURL:      req.ExternalURL,
	}
	if req.ClientSlug != "" && a.ClientService != nil {
		if cl, _ := a.ClientService.Get(r.Context(), orgID, req.ClientSlug); cl != nil {
			cid := cl.ID
			in.ClientID = &cid
		}
	}
	if req.AssigneeID != "" {
		if aid, err := uuid.Parse(req.AssigneeID); err == nil {
			in.AssigneeID = &aid
		}
	}
	if req.ParentID != "" {
		if pid, err := uuid.Parse(req.ParentID); err == nil {
			in.ParentID = &pid
		}
	}
	if req.DueDate != "" {
		if t, err := time.Parse("2006-01-02", req.DueDate); err == nil {
			in.DueDate = &t
		}
	}
	t, err := a.TicketService.Create(r.Context(), in)
	if errors.Is(err, ticketsvc.ErrExternalAlreadyLinked) {
		writeError(w, http.StatusConflict, "external_already_linked", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "create_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, t)
}

// GET /api/v1/tickets — filtros via query string
func (a *API) listTickets(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	q := r.URL.Query()
	filter := ticketsvc.ListFilter{
		Status:    q.Get("status"),
		IssueType: q.Get("issue_type"),
		Priority:  q.Get("priority"),
		Label:     q.Get("label"),
		Query:     q.Get("query"),
	}
	if slug := q.Get("project_slug"); slug != "" {
		if proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, slug); err == nil {
			pid := proj.ID
			filter.ProjectID = &pid
		}
	}
	if v := q.Get("assignee_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.AssigneeID = &id
		}
	}
	if v := q.Get("reporter_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.ReporterID = &id
		}
	}
	if v := q.Get("parent_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.ParentID = &id
		}
	}
	if v := q.Get("limit"); v != "" {
		filter.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("offset"); v != "" {
		filter.Offset, _ = strconv.Atoi(v)
	}
	list, total, err := a.TicketService.List(r.Context(), orgID, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": list, "total": total})
}

// GET /api/v1/tickets/{id_or_key}
// id_or_key puede ser UUID o KEY=ACMEWEB-1 (en cuyo caso requiere ?project_slug=)
func (a *API) getTicket(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	idOrKey := r.PathValue("id_or_key")
	if id, err := uuid.Parse(idOrKey); err == nil {
		t, err := a.TicketService.Get(r.Context(), orgID, id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", idOrKey)
			return
		}
		writeData(w, http.StatusOK, t)
		return
	}

	slug := r.URL.Query().Get("project_slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "project_slug_required_for_key", "")
		return
	}
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", slug)
		return
	}
	t, err := a.TicketService.GetByKey(r.Context(), orgID, proj.ID, idOrKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", idOrKey)
		return
	}
	writeData(w, http.StatusOK, t)
}

type updateTicketReq struct {
	Title          *string   `json:"title,omitempty"`
	DescriptionMD  *string   `json:"description_md,omitempty"`
	IssueType      *string   `json:"issue_type,omitempty"`
	Priority       *string   `json:"priority,omitempty"`
	AssigneeID     *string   `json:"assignee_id,omitempty"`
	Labels         *[]string `json:"labels,omitempty"`
	ParentID       *string   `json:"parent_id,omitempty"`
	EstimatedHours *float64  `json:"estimated_hours,omitempty"`
	ActualHours    *float64  `json:"actual_hours,omitempty"`
	DueDate        *string   `json:"due_date,omitempty"`
}

func (a *API) updateTicket(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	var req updateTicketReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	in := ticketsvc.UpdateInput{
		Title:          req.Title,
		DescriptionMD:  req.DescriptionMD,
		IssueType:      req.IssueType,
		Priority:       req.Priority,
		Labels:         req.Labels,
		EstimatedHours: req.EstimatedHours,
		ActualHours:    req.ActualHours,
	}
	if req.AssigneeID != nil {
		if aid, err := uuid.Parse(*req.AssigneeID); err == nil {
			in.AssigneeID = &aid
		}
	}
	if req.ParentID != nil {
		if pid, err := uuid.Parse(*req.ParentID); err == nil {
			in.ParentID = &pid
		}
	}
	if req.DueDate != nil {
		if t, err := time.Parse("2006-01-02", *req.DueDate); err == nil {
			in.DueDate = &t
		}
	}
	t, err := a.TicketService.Update(r.Context(), orgID, id, in)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "update_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, t)
}

// DELETE /api/v1/tickets/{id}
func (a *API) deleteTicket(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	if err := a.TicketService.Delete(r.Context(), orgID, id); err != nil {
		writeError(w, http.StatusNotFound, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/tickets/{id}/status
type changeStatusReq struct {
	ToStatus string `json:"to_status"`
	Note     string `json:"note"`
}

func (a *API) changeTicketStatus(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	userID, _ := uuid.Parse(p.UserID)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	var req changeStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	t, err := a.TicketService.ChangeStatus(r.Context(), orgID, id, req.ToStatus, userID, req.Note)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "change_status_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, t)
}

// GET /api/v1/tickets/{id}/comments
func (a *API) listTicketComments(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	cs, err := a.TicketService.ListComments(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": cs, "total": len(cs)})
}

// POST /api/v1/tickets/{id}/comments
type addCommentReq struct {
	BodyMD string `json:"body_md"`
}

func (a *API) addTicketComment(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	userID, _ := uuid.Parse(p.UserID)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	var req addCommentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	c, err := a.TicketService.AddComment(r.Context(), id, userID, req.BodyMD)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "comment_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, c)
}

// GET /api/v1/tickets/{id}/history
func (a *API) listTicketStatusHistory(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	hist, err := a.TicketService.StatusHistory(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "history_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": hist, "total": len(hist)})
}

// POST /api/v1/tickets/{id}/link-external
// body: { provider, external_id, external_url }  o {} para unlink
type linkExternalReq struct {
	Provider    string `json:"provider"`
	ExternalID  string `json:"external_id"`
	ExternalURL string `json:"external_url"`
}

func (a *API) linkTicketExternal(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	var req linkExternalReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if strings.TrimSpace(req.Provider) == "" {
		if err := a.TicketService.UnlinkExternal(r.Context(), orgID, id); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "unlink_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	t, err := a.TicketService.LinkExternal(r.Context(), orgID, id, ticketsvc.ExternalLink{
		Provider: req.Provider, ID: req.ExternalID, URL: req.ExternalURL,
	})
	if errors.Is(err, ticketsvc.ErrExternalAlreadyLinked) {
		writeError(w, http.StatusConflict, "external_already_linked", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "link_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, t)
}

// POST /api/v1/tickets/{id}/link-issue
// body: { issue_id: uuid }  o {} para desvincular
type linkIssueReq struct {
	IssueID string `json:"issue_id"`
}

func (a *API) linkTicketIssue(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	var req linkIssueReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	var issuePtr *uuid.UUID
	if strings.TrimSpace(req.IssueID) != "" {
		iID, perr := uuid.Parse(req.IssueID)
		if perr != nil {
			writeError(w, http.StatusBadRequest, "invalid_issue_id", "")
			return
		}
		issuePtr = &iID
	}
	t, err := a.TicketService.LinkIssue(r.Context(), orgID, id, issuePtr)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "link_issue_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, t)
}

// REQ-58: bulk link external. POST /api/v1/tickets/link-external-bulk
// body: { project_slug, provider, mappings: [{ticket_key|ticket_id, external_id, external_url}] }
type bulkLinkExternalReq struct {
	ProjectSlug string `json:"project_slug"`
	Provider    string `json:"provider"`
	Mappings    []struct {
		TicketID    string `json:"ticket_id,omitempty"`
		TicketKey   string `json:"ticket_key,omitempty"`
		ExternalID  string `json:"external_id"`
		ExternalURL string `json:"external_url,omitempty"`
	} `json:"mappings"`
}

func (a *API) bulkLinkTicketsExternal(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.TicketService == nil {
		writeError(w, http.StatusServiceUnavailable, "ticket_service_unavailable", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	var req bulkLinkExternalReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.ProjectSlug == "" || req.Provider == "" || len(req.Mappings) == 0 {
		writeError(w, http.StatusBadRequest, "missing_fields", "project_slug, provider, mappings (no vacio)")
		return
	}
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, req.ProjectSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", req.ProjectSlug)
		return
	}
	mappings := make([]ticketsvc.BulkLinkMapping, 0, len(req.Mappings))
	for _, m := range req.Mappings {
		mp := ticketsvc.BulkLinkMapping{
			TicketKey: m.TicketKey, ExternalID: m.ExternalID, ExternalURL: m.ExternalURL,
		}
		if m.TicketID != "" {
			if id, perr := uuid.Parse(m.TicketID); perr == nil {
				mp.TicketID = id
			}
		}
		mappings = append(mappings, mp)
	}
	res, err := a.TicketService.BulkLinkExternal(r.Context(), orgID, proj.ID, req.Provider, mappings)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "bulk_link_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, res)
}
