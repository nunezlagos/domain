package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/outboundwebhook"
)

type createOutboundWebhookReq struct {
	Name    string          `json:"name"`
	URL     string          `json:"url"`
	Events  []string        `json:"events"`
	Filters json.RawMessage `json:"filters,omitempty"`
	Secret  string          `json:"secret,omitempty"`
}

// POST /api/v1/outbound-webhooks
func (a *API) createOutboundWebhook(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.OutboundWebhookService == nil {
		writeError(w, http.StatusServiceUnavailable, "outbound_webhooks_not_configured", "")
		return
	}
	var in createOutboundWebhookReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	sub, err := a.OutboundWebhookService.Create(r.Context(), orgID,
		outboundwebhook.CreateInput{
			Name:    in.Name,
			URL:     in.URL,
			Events:  in.Events,
			Filters: in.Filters,
			Secret:  in.Secret,
		}, a.OutboundWebhookRequireTLS)
	if err != nil {
		switch {
		case errors.Is(err, outboundwebhook.ErrInvalidURL):
			writeError(w, http.StatusUnprocessableEntity, "invalid_url", "")
		case errors.Is(err, outboundwebhook.ErrSSRF):
			writeError(w, http.StatusUnprocessableEntity, "ssrf_blocked",
				"private/internal hosts not allowed")
		case errors.Is(err, outboundwebhook.ErrNoEvents):
			writeError(w, http.StatusUnprocessableEntity, "events_required", "")
		case errors.Is(err, outboundwebhook.ErrInvalidEvent):
			writeError(w, http.StatusUnprocessableEntity, "invalid_event", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/outbound-webhooks/"+sub.ID.String())
	writeData(w, http.StatusCreated, sub)
}

// GET /api/v1/outbound-webhooks
func (a *API) listOutboundWebhooks(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.OutboundWebhookService == nil {
		writeError(w, http.StatusServiceUnavailable, "outbound_webhooks_not_configured", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	subs, err := a.OutboundWebhookService.ListAll(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, subs)
}

// GET /api/v1/outbound-webhooks/{id}
func (a *API) getOutboundWebhook(w http.ResponseWriter, r *http.Request) {
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
	sub, err := a.OutboundWebhookService.Get(r.Context(), orgID, id)
	if errors.Is(err, outboundwebhook.ErrUnknown) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, sub)
}

// DELETE /api/v1/outbound-webhooks/{id}
func (a *API) deleteOutboundWebhook(w http.ResponseWriter, r *http.Request) {
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
	if err := a.OutboundWebhookService.Delete(r.Context(), orgID, id); err != nil {
		if errors.Is(err, outboundwebhook.ErrUnknown) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/outbound-webhooks/{id}/test → emite webhook.test_ping
// POST /api/v1/outbound-webhooks/deliveries/{id}/replay — issue-10.4 ow-010:
// re-encola un delivery (incluso dead_letter) con ciclo de reintentos fresco.
func (a *API) replayOutboundDelivery(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	tag, err := a.OutboundWebhookService.Pool.Exec(r.Context(), `
		UPDATE webhook_outbound_deliveries
		SET status = 'pending', next_retry_at = NOW(), attempt = 1, error_message = NULL
		WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "replay", err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	writeData(w, http.StatusAccepted, map[string]any{"replayed": true, "delivery_id": id})
}

func (a *API) testOutboundWebhook(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.OutboundWebhookDispatcher == nil {
		writeError(w, http.StatusServiceUnavailable, "dispatcher_not_configured", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	if _, err := a.OutboundWebhookService.Get(r.Context(), orgID, id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	data, _ := json.Marshal(map[string]any{"subscription_id": id, "test": true})
	ev := outboundwebhook.Event{
		ID:         uuid.New(),
		Type:       "webhook.test_ping",
		OccurredAt: time.Now().UTC(),
		Data:       data,
	}
	if err := a.OutboundWebhookDispatcher.Emit(r.Context(), orgID, ev); err != nil {
		writeError(w, http.StatusInternalServerError, "emit", err.Error())
		return
	}
	writeData(w, http.StatusAccepted, map[string]any{"queued": true, "event_id": ev.ID})
}
