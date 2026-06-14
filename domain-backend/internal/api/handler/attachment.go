package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	attSvc "nunezlagos/domain/internal/service/attachment"
)

type initUploadBody struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Filename   string `json:"filename"`
	MimeType   string `json:"mime_type"`
	SizeBytes  int64  `json:"size_bytes"`
	CreatedBy  string `json:"created_by,omitempty"`
}

// initUpload POST /api/v1/attachments
func (a *API) initUpload(w http.ResponseWriter, r *http.Request) {
	if a.AttachmentService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	var b initUploadBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	result, err := a.AttachmentService.InitUpload(r.Context(), b.EntityType, b.EntityID, b.Filename, b.MimeType, b.CreatedBy, b.SizeBytes)
	if err != nil {
		switch {
		case errors.Is(err, attSvc.ErrTooLarge):
			writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", err.Error())
		case errors.Is(err, attSvc.ErrTypeNotAllowed):
			writeError(w, http.StatusUnprocessableEntity, "type_not_allowed", err.Error())
		case errors.Is(err, attSvc.ErrInvalidEntity):
			writeError(w, http.StatusBadRequest, "invalid_entity", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "init_upload_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusCreated, result)
}

// confirmUpload POST /api/v1/attachments/{id}/confirm
func (a *API) confirmUpload(w http.ResponseWriter, r *http.Request) {
	if a.AttachmentService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	result, err := a.AttachmentService.ConfirmUpload(r.Context(), id)
	if errors.Is(err, attSvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "confirm_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, result)
}

// getAttachmentDownload GET /api/v1/attachments/{id}/download
func (a *API) getAttachmentDownload(w http.ResponseWriter, r *http.Request) {
	if a.AttachmentService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	result, err := a.AttachmentService.GetDownloadURL(r.Context(), id)
	if errors.Is(err, attSvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_download_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, result)
}

// listAttachments GET /api/v1/attachments
func (a *API) listAttachments(w http.ResponseWriter, r *http.Request) {
	if a.AttachmentService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	entityType := r.URL.Query().Get("entity_type")
	entityID := r.URL.Query().Get("entity_id")
	if entityType == "" || entityID == "" {
		writeError(w, http.StatusBadRequest, "missing_entity", "entity_type and entity_id required")
		return
	}
	attachments, err := a.AttachmentService.ListByEntity(r.Context(), entityType, entityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, attachments)
}

// deleteAttachment DELETE /api/v1/attachments/{id}
func (a *API) deleteAttachment(w http.ResponseWriter, r *http.Request) {
	if a.AttachmentService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	if err := a.AttachmentService.Delete(r.Context(), id); errors.Is(err, attSvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
