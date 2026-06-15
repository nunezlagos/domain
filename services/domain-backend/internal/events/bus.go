// Package events: in-process pub/sub para broadcastear cambios del
// dominio (ticket claim/release/reassign/update/status) a SSE
// suscriptores. REQ-69.
//
// Diseño:
//   - Bus singleton inyectado vía Deps. NO usamos channels globales —
//     todas las suscripciones se mantienen en un map[orgID][]Subscriber.
//   - Cada subscriber recibe Events SOLO de su org. Aislamiento RLS
//     extendido a streaming (mismo concepto).
//   - Topic = "ticket.claim" | "ticket.release" | "ticket.reassign" |
//             "ticket.update" | "ticket.status".
//   - Si un canal de subscriber está full, se descarta el evento para
//     ese subscriber (lossy). El SSE handler lleva su propio buffer.
//   - Heartbeat lo maneja el handler (no el bus).
package events

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Event minimo. Topic identifica qué cambió; Payload son detalles.
// CreatedAt para que el cliente sepa cuándo (no confiar en clock local).
type Event struct {
	OrgID     uuid.UUID      `json:"-"`
	Topic     string         `json:"topic"`
	TicketID  *uuid.UUID     `json:"ticket_id,omitempty"`
	ActorID   *uuid.UUID     `json:"actor_id,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Subscriber: una entrada en el bus.
type Subscriber struct {
	ID    uuid.UUID
	OrgID uuid.UUID
	Ch    chan Event
}

type Bus struct {
	mu   sync.RWMutex
	subs map[uuid.UUID][]*Subscriber // por orgID
}

func NewBus() *Bus {
	return &Bus{subs: map[uuid.UUID][]*Subscriber{}}
}

// Subscribe crea un canal buffered. El caller (SSE handler) debe llamar
// Unsubscribe al cerrar la conexión, sino leak.
func (b *Bus) Subscribe(orgID uuid.UUID, bufferSize int) *Subscriber {
	if bufferSize <= 0 {
		bufferSize = 32
	}
	s := &Subscriber{
		ID:    uuid.New(),
		OrgID: orgID,
		Ch:    make(chan Event, bufferSize),
	}
	b.mu.Lock()
	b.subs[orgID] = append(b.subs[orgID], s)
	b.mu.Unlock()
	return s
}

func (b *Bus) Unsubscribe(s *Subscriber) {
	if s == nil {
		return
	}
	b.mu.Lock()
	list := b.subs[s.OrgID]
	out := list[:0]
	for _, x := range list {
		if x.ID != s.ID {
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		delete(b.subs, s.OrgID)
	} else {
		b.subs[s.OrgID] = out
	}
	b.mu.Unlock()
	close(s.Ch)
}

// Publish: lossy fan-out. NO bloquea si un subscriber está full —
// descarta el evento para ese subscriber y continúa con los demás.
// Eso protege al backend de un cliente lento.
func (b *Bus) Publish(ev Event) {
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now()
	}
	b.mu.RLock()
	list := b.subs[ev.OrgID]
	subs := make([]*Subscriber, len(list))
	copy(subs, list)
	b.mu.RUnlock()
	for _, s := range subs {
		select {
		case s.Ch <- ev:
		default:
			// canal full → drop. El cliente puede recuperarse del
			// estado real via GET (eventos son notificaciones, no
			// fuente de verdad).
		}
	}
}

// PublishCtx variant que aborta si el ctx se cancela mid-fanout.
// Útil cuando el caller corre dentro de un request handler.
func (b *Bus) PublishCtx(ctx context.Context, ev Event) {
	if err := ctx.Err(); err != nil {
		return
	}
	b.Publish(ev)
}

// Stats expone contadores para /metrics (REQ-70 lo consume después).
func (b *Bus) Stats() (totalSubs int, perOrg map[uuid.UUID]int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	perOrg = map[uuid.UUID]int{}
	for org, list := range b.subs {
		perOrg[org] = len(list)
		totalSubs += len(list)
	}
	return
}
