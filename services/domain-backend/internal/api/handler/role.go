package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/auth/rbac"
	rolesvc "nunezlagos/domain/internal/service/role"
)

type createRoleBody struct {
	Slug        string                 `json:"slug"`
	Name        string                 `json:"name"`
	Permissions map[string]interface{} `json:"permissions"`
	Description string                 `json:"description,omitempty"`
}

type updateRoleBody struct {
	Name        *string                `json:"name,omitempty"`
	Permissions map[string]interface{} `json:"permissions,omitempty"`
	Description *string                `json:"description,omitempty"`
}

type assignRoleBody struct {
	Role string `json:"role"`
}

// listRoles GET /api/v1/organizations/{id}/roles
func (a *API) listRoles(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org_id", "")
		return
	}
	if a.RoleService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	roles, err := a.RoleService.ListRoles(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, roles)
}

// createRole POST /api/v1/organizations/{id}/roles
func (a *API) createRole(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org_id", "")
		return
	}
	if a.RoleService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}

	var b createRoleBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	actorID, _ := uuid.Parse(p.UserID)
	role, err := a.RoleService.CreateRole(r.Context(), orgID, actorID, b.Slug, b.Name, b.Description, b.Permissions)
	if err != nil {
		var valErr *rbac.ValidationError
		if errors.As(err, &valErr) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", valErr.Error())
			return
		}
		switch {
		case errors.Is(err, rolesvc.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug_taken", err.Error())
		case errors.Is(err, rolesvc.ErrBuiltinRole):
			writeError(w, http.StatusForbidden, "builtin_immutable", err.Error())
		case errors.Is(err, rolesvc.ErrSlugRequired), errors.Is(err, rolesvc.ErrNameRequired), errors.Is(err, rolesvc.ErrPermsRequired):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusCreated, role)
}

// getRole GET /api/v1/organizations/{id}/roles/{slug}
func (a *API) getRole(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org_id", "")
		return
	}
	if a.RoleService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	role, err := a.RoleService.GetRoleBySlug(r.Context(), orgID, slug)
	if errors.Is(err, rolesvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "role not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, role)
}

// updateRole PATCH /api/v1/organizations/{id}/roles/{slug}
func (a *API) updateRole(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org_id", "")
		return
	}
	if a.RoleService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")

	var b updateRoleBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	actorID, _ := uuid.Parse(p.UserID)
	role, err := a.RoleService.UpdateRole(r.Context(), orgID, actorID, slug, b.Name, b.Permissions, b.Description)
	if err != nil {
		var valErr *rbac.ValidationError
		if errors.As(err, &valErr) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", valErr.Error())
			return
		}
		switch {
		case errors.Is(err, rolesvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, rolesvc.ErrBuiltinRole):
			writeError(w, http.StatusForbidden, "builtin_immutable", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, role)
}

// deleteRole DELETE /api/v1/organizations/{id}/roles/{slug}
func (a *API) deleteRole(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org_id", "")
		return
	}
	if a.RoleService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")

	actorID, _ := uuid.Parse(p.UserID)
	err = a.RoleService.DeleteRole(r.Context(), orgID, actorID, slug)
	if err != nil {
		switch {
		case errors.Is(err, rolesvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, rolesvc.ErrBuiltinRole):
			writeError(w, http.StatusForbidden, "builtin_immutable", err.Error())
		case errors.Is(err, rolesvc.ErrHasMembers):
			writeError(w, http.StatusConflict, "role_has_members", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// assignRole POST /api/v1/organizations/{id}/members/{user_id}/role
func (a *API) assignRole(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org_id", "")
		return
	}
	if a.RoleService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	targetUserID, err := uuid.Parse(r.PathValue("user_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_user_id", "")
		return
	}

	var b assignRoleBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	actorID, _ := uuid.Parse(p.UserID)
	err = a.RoleService.AssignRole(r.Context(), orgID, actorID, targetUserID, b.Role)
	if err != nil {
		switch {
		case errors.Is(err, rolesvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "assign_failed", err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}


