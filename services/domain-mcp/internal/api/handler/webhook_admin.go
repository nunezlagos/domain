// Handlers admin de webhooks inbound — issue-10.2 (CRUD + deliveries + replay).
// El secret se revela UNA sola vez en el response del create; después solo
// existe cifrado at-rest.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	websvc "nunezlagos/domain/internal/service/webhook"
)

type createWebhookRequest struct {
	Slug          string         `json:"slug"`
	Name          string         `json:"name"`
	Secret        string         `json:"secret"`
	SourceType    string         `json:"source_type"`
	TargetType    string         `json:"target_type"`
	TargetID      uuid.UUID      `json:"target_id"`
	InputsMapping map[string]any `json:"inputs_mapping"`
}

func (a *API) createInboundWebhook(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.WebhookService == nil {
		writeError(w, http.StatusServiceUnavailable, "webhooks_disabled", "")
		return
	}
	var req createWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if req.Secret == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "secret required")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	actorID, _ := uuid.Parse(p.UserID)
	out, err := a.WebhookService.Create(r.Context(), websvc.CreateInput{
		OrganizationID: orgID, CreatedBy: &actorID,
		Slug: req.Slug, Name: req.Name, Secret: req.Secret,
		SourceType: req.SourceType, TargetType: req.TargetType,
		TargetID: req.TargetID, InputsMapping: req.InputsMapping,
		ActorID: actorID,
	})
	switch {
	case errors.Is(err, websvc.ErrSlugTaken):
		writeError(w, http.StatusConflict, "slug_taken", err.Error())
		return
	case errors.Is(err, websvc.ErrSlugInvalid),
		errors.Is(err, websvc.ErrInvalidSourceType),
		errors.Is(err, websvc.ErrInvalidTargetType):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "create", err.Error())
		return
	}
	w.Header().Set("Location", "/api/v1/inbound-webhooks/"+out.ID.String())
	writeData(w, http.StatusCreated, map[string]any{
		"webhook":     out,
		"receive_url": "/api/v1/webhooks/" + out.Slug + "/receive",
	})
}

func (a *API) listInboundWebhooks(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	out, err := a.WebhookService.List(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	if out == nil {
		out = []websvc.Webhook{}
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) lookupInboundWebhook(w http.ResponseWriter, r *http.Request) *websvc.Webhook {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return nil
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return nil
	}
	hook, err := a.WebhookService.GetByID(r.Context(), id)
	if errors.Is(err, websvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return nil
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return nil
	}
	return hook
}

func (a *API) getInboundWebhook(w http.ResponseWriter, r *http.Request) {
	if hook := a.lookupInboundWebhook(w, r); hook != nil {
		writeData(w, http.StatusOK, hook)
	}
}

func (a *API) patchInboundWebhook(w http.ResponseWriter, r *http.Request) {
	hook := a.lookupInboundWebhook(w, r)
	if hook == nil {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if req.Enabled == nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "enabled required")
		return
	}
	if err := a.WebhookService.SetEnabled(r.Context(), hook.ID, *req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	hook.Enabled = *req.Enabled
	writeData(w, http.StatusOK, hook)
}

func (a *API) deleteInboundWebhook(w http.ResponseWriter, r *http.Request) {
	hook := a.lookupInboundWebhook(w, r)
	if hook == nil {
		return
	}
	p, _ := principal(r)
	actorID, _ := uuid.Parse(p.UserID)
	if err := a.WebhookService.SoftDelete(r.Context(), hook.ID, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	hook := a.lookupInboundWebhook(w, r)
	if hook == nil {
		return
	}
	out, err := a.WebhookService.Deliveries(r.Context(), hook.ID, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "deliveries", err.Error())
		return
	}
	if out == nil {
		out = []websvc.Delivery{}
	}
	writeData(w, http.StatusOK, out)
}

// POST /api/v1/inbound-webhooks/deliveries/{id}/replay — re-dispatcha el
// payload almacenado contra el target actual del webhook (ciclo fresco,
// nueva entrada en deliveries).
func (a *API) replayWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	d, err := a.WebhookService.GetDelivery(r.Context(), id)
	if errors.Is(err, websvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	hook, err := a.WebhookService.GetByID(r.Context(), d.WebhookID)
	if errors.Is(err, websvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}

	body, _ := json.Marshal(d.Payload)
	inputs := buildInputs(body, hook.InputsMapping)
	headers := map[string]string{"X-Domain-Replay-Of": d.ID.String()}
	go a.runWebhookTarget(context.Background(), hook, body, inputs, headers, "replay")

	writeData(w, http.StatusAccepted, map[string]any{
		"replayed":   true,
		"webhook_id": hook.ID,
		"of":         d.ID,
	})
}
