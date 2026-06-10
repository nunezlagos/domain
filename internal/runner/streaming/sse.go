// HU-11.3 execution-streaming — SSE (Server-Sent Events) para streamear
// progreso/output de runs a clientes en tiempo real.
//
// Compatible con cualquier runner: agent, flow, skill. Cada chunk emitido
// se escribe al ResponseWriter con format event-stream.
package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// EventType de cada chunk SSE.
const (
	EventStarted   = "started"
	EventChunk     = "chunk"
	EventToolUse   = "tool_use"
	EventCompleted = "completed"
	EventError     = "error"
	EventHeartbeat = "heartbeat"
)

// Chunk es un evento a enviar.
type Chunk struct {
	Type    string         `json:"type"`
	Data    any            `json:"data,omitempty"`
	RunID   string         `json:"run_id,omitempty"`
	StepKey string         `json:"step_key,omitempty"`
	At      time.Time      `json:"at"`
}

// Stream maneja una conexión SSE con un cliente.
type Stream struct {
	w           http.ResponseWriter
	flusher     http.Flusher
	mu          sync.Mutex
	closed      bool
	heartbeatT  *time.Ticker
}

// New construye un Stream sobre el ResponseWriter. Setea headers SSE.
// Retorna error si ResponseWriter no soporta Flusher (http.Hijacker required).
func New(w http.ResponseWriter) (*Stream, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("ResponseWriter no soporta flush; SSE no disponible")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx: deshabilita buffering proxy
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	s := &Stream{w: w, flusher: flusher}
	return s, nil
}

// Send escribe un chunk al stream. Thread-safe.
func (s *Stream) Send(ch Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("stream closed")
	}
	if ch.At.IsZero() {
		ch.At = time.Now()
	}
	body, err := json.Marshal(ch)
	if err != nil {
		return fmt.Errorf("marshal chunk: %w", err)
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", ch.Type, body); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	s.flusher.Flush()
	return nil
}

// StartHeartbeat envía un keepalive cada interval para mantener la conexión
// viva a través de proxies con idle timeout. Detenete vía Close.
func (s *Stream) StartHeartbeat(interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	s.mu.Lock()
	if s.heartbeatT != nil {
		s.mu.Unlock()
		return
	}
	s.heartbeatT = time.NewTicker(interval)
	s.mu.Unlock()

	go func() {
		for range s.heartbeatT.C {
			_ = s.Send(Chunk{Type: EventHeartbeat})
		}
	}()
}

// Close marca el stream como cerrado y detiene heartbeats.
func (s *Stream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.heartbeatT != nil {
		s.heartbeatT.Stop()
	}
}

// Pump consume chunks de un channel y los envía al stream hasta que el
// context se cancele o el channel cierre.
func (s *Stream) Pump(ctx context.Context, in <-chan Chunk) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch, ok := <-in:
			if !ok {
				return nil
			}
			if err := s.Send(ch); err != nil {
				return err
			}
		}
	}
}
