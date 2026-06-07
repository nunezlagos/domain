# Tasks: HU-11.3-execution-streaming

## Backend

- [ ] Implementar `StreamManager` singleton con mapa de RunStreams
- [ ] Implementar `RunStream` struct con subscribers map, ring buffer, mutex
- [ ] Implementar `Event` struct: ID (int64 auto-incremental por run), Type, Payload, Timestamp
- [ ] Implementar ring buffer circular de 1000 eventos por run
- [ ] Implementar `Subscribe(runID, ch)` y `Unsubscribe(runID, ch)`
- [ ] Implementar `Publish(runID, event)` con broadcast a todos los subscribers + buffer append
- [ ] Implementar `Replay(runID, lastEventID)` que envía eventos desde buffer
- [ ] Implementar WebSocket handler en `/ws/runs/{run_id}` con gorilla/websocket
- [ ] Implementar upgrade HTTP → WS, read/write goroutines
- [ ] Implementar SSE handler en `/api/v1/runs/{run_id}/events`
- [ ] Implementar SSE handler en `/api/v1/runs/{run_id}/progress`
- [ ] Implementar evento `token` para streaming de LLM
- [ ] Implementar evento `step_start`, `step_complete`, `step_error`
- [ ] Implementar evento `run_complete`, `run_error`
- [ ] Implementar evento `progress` con step_current, step_total, pct
- [ ] Implementar evento `log_line` para logs arbitrarios
- [ ] Integrar `StreamManager.Publish` en `RunnerService` cuando steps cambian
- [ ] Integrar `StreamManager.Publish` en LLM providers para token streaming
- [ ] Implementar cleanup: liberar RunStream 5 minutos después de run_complete
- [ ] Implementar rate limiting por IP en WS y SSE
- [ ] Implementar CORS headers para SSE
- [ ] Implementar timeout de conexiones inactivas (30 min)
- [ ] Agregar config `stream:` en config.yaml (buffer_size, cleanup_ttl, max_connections)

## Frontend

- [ ] (No aplica - es backend de streaming; frontend SDK será en REQ-14-cli y REQ-16-web-ui)

## Tests

- [ ] Test unitario: StreamManager.Publish entrega evento a un subscriber
- [ ] Test unitario: StreamManager.Publish entrega a múltiples subscribers
- [ ] Test unitario: Unsubscribe deja de recibir eventos
- [ ] Test unitario: buffer circular mantiene límite de N eventos
- [ ] Test unitario: Replay desde event_id 0 devuelve todos los eventos
- [ ] Test unitario: Replay desde event_id específico devuelve solo posteriores
- [ ] Test unitario: Publish después de Replay envía eventos en vivo
- [ ] Test unitario: cleanup libera recursos después de TTL
- [ ] Test integración: WS client se conecta y recibe eventos
- [ ] Test integración: SSE client recibe eventos con flusheo
- [ ] Test integración: reconexión con last_event_id no duplica
- [ ] Test integración: múltiples WS clients reciben mismos eventos
- [ ] Test carga: 100 conexiones simultáneas, verificar throughput y mem
- [ ] Sabotaje: no lockear subscribers map → data race en test -race
- [ ] Sabotaje: no limpiar subscribers muertos → memory leak

## Cierre

- [ ] Verificación manual: abrir WS en navegador, ejecutar flow, ver eventos
- [ ] Verificación manual: SSE con curl
- [ ] Verificación manual: reconexión con last_event_id
- [ ] Suite verde: `go test ./internal/runner/stream/... ./internal/api/...`
- [ ] Documentar API de streaming en docs/execution-streaming.md
