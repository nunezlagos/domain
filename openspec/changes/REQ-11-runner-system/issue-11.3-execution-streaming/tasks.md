# Tasks: issue-11.3-execution-streaming

## Backend

- [x] Implementar `StreamManager` singleton con mapa de RunStreams
- [x] Implementar `RunStream` struct con subscribers map, ring buffer, mutex
- [x] Implementar `Event` struct: ID (int64 auto-incremental por run), Type, Payload, Timestamp
- [x] Implementar ring buffer circular de 1000 eventos por run
- [x] Implementar `Subscribe(runID, ch)` y `Unsubscribe(runID, ch)`
- [x] Implementar `Publish(runID, event)` con broadcast a todos los subscribers + buffer append
- [x] Implementar `Replay(runID, lastEventID)` que envía eventos desde buffer
- [x] Implementar WebSocket handler en `/ws/runs/{run_id}` con gorilla/websocket
- [x] Implementar upgrade HTTP → WS, read/write goroutines
- [x] Implementar SSE handler en `/api/v1/runs/{run_id}/events`
- [x] Implementar SSE handler en `/api/v1/runs/{run_id}/progress`
- [x] Implementar evento `token` para streaming de LLM
- [x] Implementar evento `step_start`, `step_complete`, `step_error`
- [x] Implementar evento `run_complete`, `run_error`
- [x] Implementar evento `progress` con step_current, step_total, pct
- [x] Implementar evento `log_line` para logs arbitrarios
- [x] Integrar `StreamManager.Publish` en `RunnerService` cuando steps cambian
- [x] Integrar `StreamManager.Publish` en LLM providers para token streaming
- [x] Implementar cleanup: liberar RunStream 5 minutos después de run_complete
- [x] Implementar rate limiting por IP en WS y SSE
- [x] Implementar CORS headers para SSE
- [x] Implementar timeout de conexiones inactivas (30 min)
- [x] Agregar config `stream:` en config.yaml (buffer_size, cleanup_ttl, max_connections)

## Frontend

- [x] (No aplica - es backend de streaming; frontend SDK será en REQ-14-cli y REQ-16-web-ui)

## Tests

- [x] Test unitario: StreamManager.Publish entrega evento a un subscriber
- [x] Test unitario: StreamManager.Publish entrega a múltiples subscribers
- [x] Test unitario: Unsubscribe deja de recibir eventos
- [x] Test unitario: buffer circular mantiene límite de N eventos
- [x] Test unitario: Replay desde event_id 0 devuelve todos los eventos
- [x] Test unitario: Replay desde event_id específico devuelve solo posteriores
- [x] Test unitario: Publish después de Replay envía eventos en vivo
- [x] Test unitario: cleanup libera recursos después de TTL
- [x] Test integración: WS client se conecta y recibe eventos
- [x] Test integración: SSE client recibe eventos con flusheo
- [x] Test integración: reconexión con last_event_id no duplica
- [x] Test integración: múltiples WS clients reciben mismos eventos
- [x] Test carga: 100 conexiones simultáneas, verificar throughput y mem
- [x] Sabotaje: no lockear subscribers map → data race en test -race
- [x] Sabotaje: no limpiar subscribers muertos → memory leak

## Cierre

- [x] Verificación manual: abrir WS en navegador, ejecutar flow, ver eventos
- [x] Verificación manual: SSE con curl
- [x] Verificación manual: reconexión con last_event_id
- [x] Suite verde: `go test ./internal/runner/stream/... ./internal/api/...`
- [x] Documentar API de streaming en docs/execution-streaming.md
