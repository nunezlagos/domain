package admin

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	orgsvc "nunezlagos/domain/internal/service/org"
)

type OrgDeleteHandler struct {
	Pool          *pgxpool.Pool
	DeleteService *orgsvc.DeleteService
}

func NewOrgDeleteHandler(pool *pgxpool.Pool) *OrgDeleteHandler {
	return &OrgDeleteHandler{Pool: pool}
}

func (h *OrgDeleteHandler) DeleteDELETE(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Confirm") != "true" {
		writeError(w, http.StatusBadRequest, "confirm_required", "X-Confirm: true header is required")
		return
	}

	orgIDStr := r.PathValue("id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org_id", "invalid organization ID")
		return
	}

	if h.DeleteService == nil {
		h.DeleteService = orgsvc.NewDeleteService(h.Pool, nil)
	}

	actorID := uuid.Nil
	result, err := h.DeleteService.DeleteOrg(r.Context(), orgID, &actorID, "api-admin")
	if err != nil {
		if errors.Is(err, orgsvc.ErrNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}

	if result.RowsDeleted == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
