// Handler REST para clients (mandantes). Útil para consultoras que
// gestionan proyectos por cliente.
//
// Endpoints:
//   POST   /api/v1/clients
//   GET    /api/v1/clients               ?status= &limit= &offset=
//   GET    /api/v1/clients/{id_or_slug}
//   PUT    /api/v1/clients/{id}
//   DELETE /api/v1/clients/{id}          (soft-delete)
//   POST   /api/v1/clients/{id}/restore
//   POST   /api/v1/clients/{id}/status   body: {status}
//
// Convenciones:
//   - Middleware PrincipalCtx ya inyecta org/user en ctx (HU-28.3).
//   - El Service scope-ea todas las operaciones por orgID (no necesitamos
//     authorizeOrg porque Service.Get/Update/Delete ya filtran).
//   - writeJSON/writeError/writeData helpers de api.go (HU-28.5).
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	clientsvc "nunezlagos/domain/internal/service/client"
)

// --- request bodies ---

type createClientBody struct {
	Name         string         `json:"name"`
	Slug         string         `json:"slug"`
	TaxID        string         `json:"tax_id,omitempty"`
	ContactEmail string         `json:"contact_email,omitempty"`
	ContactPhone string         `json:"contact_phone,omitempty"`
	Address      string         `json:"address,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type updateClientBody struct {
	Name         *string        `json:"name,omitempty"`
	TaxID        *string        `json:"tax_id,omitempty"`
	ContactEmail *string        `json:"contact_email,omitempty"`
	ContactPhone *string        `json:"contact_phone,omitempty"`
	Address      *string        `json:"address,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type setClientStatusBody struct {
	Status string `json:"status"`
}

// --- handlers ---

func (a *API) createClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	actorID := a.userID(ctx)
	var b createClientBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.Name == "" || b.Slug == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "name y slug requeridos")
		return
	}
	out, err := a.ClientService.Create(ctx, orgID, clientsvc.CreateInput{
		Name:         b.Name,
		Slug:         b.Slug,
		TaxID:        b.TaxID,
		ContactEmail: b.ContactEmail,
		ContactPhone: b.ContactPhone,
		Address:      b.Address,
		Metadata:     b.Metadata,
		ActorID:      &actorID,
	})
	if err != nil {
		switch {
		case errors.Is(err, clientsvc.ErrInvalidSlug):
			writeError(w, http.StatusUnprocessableEntity, "invalid_slug", err.Error())
		case errors.Is(err, clientsvc.ErrClientSlugExists):
			writeError(w, http.StatusConflict, "slug_taken", err.Error())
		case errors.Is(err, clientsvc.ErrInvalidStatus):
			writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
		case errors.Is(err, clientsvc.ErrInvalidName):
			writeError(w, http.StatusUnprocessableEntity, "invalid_name", err.Error())
		case errors.Is(err, clientsvc.ErrInvalidEmail):
			writeError(w, http.StatusUnprocessableEntity, "invalid_email", err.Error())
		case errors.Is(err, clientsvc.ErrInvalidTaxID):
			writeError(w, http.StatusUnprocessableEntity, "invalid_tax_id", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_client", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/clients/"+out.Slug)
	writeData(w, http.StatusCreated, out)
}

func (a *API) listClients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	list, total, err := a.ClientService.List(ctx, orgID, clientsvc.ListFilter{
		Status: q.Get("status"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": list, "total": total})
}

// getClient acepta UUID o slug en {id_or_slug}.
func (a *API) getClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	out, err := a.ClientService.Get(ctx, orgID, r.PathValue("id_or_slug"))
	if err != nil {
		if errors.Is(err, clientsvc.ErrClientNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) updateClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	var b updateClientBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	actorID := a.userID(ctx)
	out, err := a.ClientService.Update(ctx, orgID, id, clientsvc.UpdateInput{
		Name:         b.Name,
		TaxID:        b.TaxID,
		ContactEmail: b.ContactEmail,
		ContactPhone: b.ContactPhone,
		Address:      b.Address,
		Metadata:     b.Metadata,
		ActorID:      &actorID,
	})
	if err != nil {
		if errors.Is(err, clientsvc.ErrClientNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) deleteClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err := a.ClientService.Delete(ctx, orgID, id); err != nil {
		if errors.Is(err, clientsvc.ErrClientNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) restoreClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err := a.ClientService.Restore(ctx, orgID, id); err != nil {
		if errors.Is(err, clientsvc.ErrClientNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "restore", err.Error())
		return
	}
	out, err := a.ClientService.Get(ctx, orgID, id.String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_after_restore", err.Error())
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) setClientStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	var b setClientStatusBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.Status == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "status requerido")
		return
	}
	out, err := a.ClientService.SetStatus(ctx, orgID, id, b.Status)
	if err != nil {
		switch {
		case errors.Is(err, clientsvc.ErrClientNotFound):
			writeError(w, http.StatusNotFound, "not_found", "")
		case errors.Is(err, clientsvc.ErrInvalidStatus):
			writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "set_status", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, out)
}
