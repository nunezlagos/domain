package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/events"
)

// REQ-69: GET /api/v1/events
//
// Server-Sent Events stream. El cliente se conecta con su API key
// (Bearer) y recibe eventos del bus filtrados a su org.
//
// Eventos broadcasteados hoy:
//   - ticket.claim     (payload: {ticket_id, locked_by, locked_until})
//   - ticket.release   (payload: {ticket_id})
//   - ticket.reassign  (payload: {ticket_id, old_assignee, new_assignee})
//   - ticket.update    (payload: {ticket_id, changes:[]})
//   - ticket.status    (payload: {ticket_id, from, to})
//
// Heartbeat: cada 25s manda comentario ": ping\n\n" para mantener
// proxies abiertos (Caddy/nginx desconectan tras ~60s idle).
//
// Reconexion: el cliente debe asumir lossy (no buffer historico).
// Para sincronizarse despues de reconectar: GET el recurso afectado.
func (a *API) sseEvents(w http.ResponseWriter, r *http.Request) {
	if a.EventBus == nil {
		http.Error(w, "events bus no configurado", http.StatusServiceUnavailable)
		return
	}
	princ, ok := apikey.FromContext(r.Context())
	if !ok || princ == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	orgID, err := uuid.Parse(princ.OrganizationID)
	if err != nil {
		http.Error(w, "principal org invalido", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming no soportado por el writer", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)

	sub := a.EventBus.Subscribe(orgID, 64)
	defer a.EventBus.Unsubscribe(sub)


	fmt.Fprintf(w, "event: hello\ndata: {\"sub_id\":\"%s\"}\n\n", sub.ID)
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, ok := <-sub.Ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Topic, payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// publishTicketEvent helper para que los handlers REST emitan eventos.
// El handler MCP equivalente lo hace via service hook (REQ-69 hook layer).
func (a *API) publishTicketEvent(topic string, orgID, ticketID uuid.UUID, actor *uuid.UUID, payload map[string]any) {
	if a.EventBus == nil {
		return
	}
	a.EventBus.Publish(events.Event{
		OrgID:    orgID,
		Topic:    topic,
		TicketID: &ticketID,
		ActorID:  actor,
		Payload:  payload,
	})
}
