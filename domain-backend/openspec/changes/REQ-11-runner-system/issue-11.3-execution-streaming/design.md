# Design: issue-11.3-execution-streaming

## Decisión arquitectónica

```
                    ┌─────────────────────┐
                    │   StreamManager     │
                    │  (singleton)        │
                    │                     │
                    │  map[string]*       │
                    │    RunStream        │
                    └──────┬──────────────┘
                           │ Publish(runID, event)
                           │
              ┌────────────┼────────────┐
              │            │            │
         ┌────▼───┐  ┌────▼───┐  ┌────▼───┐
         │RunStream│  │RunStream│  │RunStream│
         │(run_abc)│  │(run_def)│  │(run_ghi)│
         │         │  │         │  │         │
         │subs map │  │subs map │  │subs map │
         │buffer[] │  │buffer[] │  │buffer[] │
         └────┬────┘  └─────────┘  └─────────┘
              │
     ┌────────┼────────┬────────┐
     │        │        │        │
  ┌──▼─┐  ┌──▼─┐  ┌──▼─┐  ┌──▼─┐
  │ WS │  │ WS │  │ SSE│  │ SSE│
  │ cl1│  │ cl2│  │ cl3│  │ cl4│
  └────┘  └────┘  └────┘  └────┘
```

**Decisión:** WebSocket para streaming bidireccional detallado (el cliente necesita enviar last_event_id, confirmaciones). SSE para consumo unidireccional ligero (progreso, eventos de UI). StreamManager es el hub central que desacopla productores (RunnerService, LLM providers) de consumidores.

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|---|---|
| gRPC server-streaming | Requiere cliente gRPC, no disponible desde browsers nativos |
| Server-Sent Events puro | No permite enviar datos del cliente (last_event_id en query params es workaround) |
| WebSocket puro | Más pesado que SSE para progreso simple que solo lectura |
| Redis Pub/Sub | Overkill si solo hay un servidor; se考虑 para multi-instancia futura |
| MQTT | Protocolo no estándar en web, requiere librerías adicionales |

## Diagrama

```
─── FLUJO DE STREAMING ───

RunnerService                    StreamManager                    Cliente WebSocket
    │                                │                                │
    │──── step_complete ────────────►│                                │
    │    {run_id, step_id, ...}      │── broadcast ─────────────────►│
    │                                │    {type:"step_complete",...} │
    │                                │── buffer.append(event)        │
    │                                │                                │
    │──── token ────────────────────►│                                │
    │    {run_id, text, step_id}     │── broadcast ─────────────────►│
    │                                │    {type:"token",text:"The"}  │
    │                                │                                │

─── FLUJO DE RECONEXIÓN ───

Cliente                            StreamManager
    │                                │
    │──── WS CONNECT ───────────────►│
    │    /ws/runs/run_abc            │
    │    ?last_event_id=42          │
    │                                │── buscar run en mapa
    │                                │── replay events 43..N
    │◄─── replay batch ─────────────│
    │    [{id:43,...},{id:44,...}]  │
    │                                │
    │◄─── live streaming ───────────│
    │    (eventos nuevos)           │
```

## TDD plan

1. **Red:** Test que StreamManager.Publish envía evento a subscribers
2. **Green:** Implementar StreamManager con map + broadcast
3. **Refactor:** Extraer RunStream como struct separado
4. **Red:** Test que subscriber recibe evento después de suscribirse
5. **Green:** Implementar Subscribe/Unsubscribe en RunStream
6. **Red:** Test que buffer circular mantiene últimos N eventos
7. **Green:** Implementar ring buffer
8. **Red:** Test que replay desde event_id devuelve eventos correctos
9. **Green:** Implementar Replay(lastEventID) en RunStream
10. **Red:** Test WebSocket upgrade y mensajes entrantes/salientes
11. **Green:** Implementar WS handler con gorilla/websocket
12. **Red:** Test SSE endpoint escribe eventos con Flush
13. **Green:** Implementar SSE handler con http.Flusher
14. **Red:** Test que cleanup libera recursos después de TTL
15. **Green:** Implementar RunStream.Cleanup() con time.AfterFunc
16. **Sabotaje:** No llamar a Flush en SSE → eventos se bufferizan

## Riesgos y mitigación

- **Mem leak por subscribers muertos:** Goroutine detector de conexiones muertas, Unsubscribe on error/timeout
- **Buffer overflow:** Máximo 1000 eventos por run, si se excede se descartan los más viejos (ring buffer)
- **Race conditions:** Todos los accesos a RunStream con sync.RWMutex
- **Proxy buffering SSE:** Headers `X-Accel-Buffering: no`, `Cache-Control: no-cache`, `Connection: keep-alive`
- **Many connections:** Limitar por IP, máximo global configurable, rechazar con 429 si se excede
