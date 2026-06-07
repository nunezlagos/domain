# Proposal: HU-11.3-execution-streaming

## Intención

Implementar streaming en tiempo real de ejecuciones (flows, skills, code_exec) mediante WebSocket y SSE. Los clientes pueden suscribirse por run_id y recibir eventos de step completion, LLM token streaming, errores y progreso general. Soporta reconexión con replay de eventos perdidos.

## Scope

**Incluye:**
- WebSocket endpoint `/ws/runs/{run_id}` para streaming detallado
- SSE endpoints: `/api/v1/runs/{run_id}/events` (eventos completos), `/api/v1/runs/{run_id}/progress` (solo progreso)
- Tipos de evento: `step_start`, `step_complete`, `step_error`, `token`, `done`, `progress`, `run_complete`, `run_error`, `log_line`
- `StreamManager`: hub central que gestiona suscripciones y broadcast
- Event buffer circular para reconexión (últimos N eventos por run_id, configurable)
- Reconexión con `?last_event_id=` para replay
- Publicación desde `RunnerService` vía `StreamManager.Publish(runID, event)`
- Soporte para múltiples clientes por run_id
- Timeout de conexiones inactivas (30min sin actividad)
- CORS headers para SSE desde web UI
- Compatibilidad con nginx/apache para SSE (buffering disabled)

**No incluye:**
- Persistencia de eventos en DB (solo buffer en memoria)
- Historial de eventos para runs completados (se puede reconstruir desde DB)
- Streaming de archivos grandes (solo texto)
- Video streaming ni binario

## Enfoque técnico

1. `StreamManager` singleton con mapa `map[string]*RunStream` (run_id → canal de eventos)
2. `RunStream` struct con: `subscribers map[string]chan Event`, `buffer *ringbuffer`, `mu sync.RWMutex`
3. WebSocket con `gorilla/websocket`: upgrade HTTP → WS, read/write goroutines por cliente
4. SSE con `http.Flusher`: eventos `text/event-stream`, flusheo después de cada evento
5. `Event` struct: ID (auto-incremental por run), Type, Payload (any), Timestamp
6. Buffer circular: anillo de 1000 eventos por run_id en memoria
7. Reconexión: cliente envía `last_event_id`, servidor replay desde buffer, luego live
8. Publicación: `streamManager.Publish(runID, event)` → broadcast a todos los subscribers + buffer append
9. Cleanup: cuando run completa, buffer se mantiene 5 minutos más, luego se libera
10. LLM streaming: los providers publican tokens como eventos `token` individuales

## Riesgos

- **Domain:** Si hay muchos runs concurrentes con muchos eventos, el buffer puede crecer. Mitigación: buffer circular con max 1000 eventos, TTL de 5 min post-completion.
- **WebSocket concurrente:** Muchas conexiones pueden saturar el servidor. Mitigación: max connections per IP, rate limiting.
- **SSE con proxies:** Nginx/Apache bufferizan por defecto. Mitigación: `X-Accel-Buffering: no` y `Cache-Control: no-cache`.
- **Orden de eventos:** LLM tokens pueden llegar fuera de orden si hay concurrencia. Mitigación: secuenciador por step_id con número de secuencia.
- **Reconexión masiva:** Muchos clientes reconectando simultáneamente. Mitigación: jitter + backoff del lado del cliente.

## Testing

- Unit: StreamManager publish/subscribe/unsubscribe
- Unit: buffer circular con límite y eviction
- Unit: replay desde event_id específico
- Integration: WebSocket client recibe eventos en tiempo real
- Integration: SSE client recibe eventos con flusheo
- Integration: reconexión con last_event_id no duplica eventos
- Integration: múltiples clientes reciben mismos eventos
- Load test: 1000 conexiones simultáneas, verificar throughput
