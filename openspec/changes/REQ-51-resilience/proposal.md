# REQ-51 — Resiliencia de conexiones (cliente -> admin -> MCP -> DB -> LLM)

> 3 HUs incrementales que cubren los 3 puntos del stack para que un
> cliente que se queda sin internet (o el admin se reinicia, o el MCP
> se cae, o MiniMax tiene un timeout) pueda retomar su trabajo sin
> perder mensajes ni quedar en estados inconsistentes.

## Motivacion

Hoy hay 3 problemas de resiliencia que afectan al usuario:

1. **Cliente (browser) -> admin**: el chat widget usa `fetch` plano. Si
   el cliente pierde internet o el admin se reinicia, el mensaje se
   pierde o queda en estado "processing" para siempre.

2. **Admin (Django) -> DB y LLM**: queries a Postgres sin `connect_timeout`,
   llamada al LLM con timeout fijo de 120s, `process_message` sincrono
   que bloquea el request HTTP. Si Postgres esta lento o el LLM cuelga,
   el cliente hace timeout y el mensaje queda "en processing" para siempre.

3. **MCP (Go) -> DB y LLM**: handlers HTTP sin `context.WithTimeout`. Si el
   cliente cierra la conexion a mitad de un request largo, el MCP sigue
   procesando (desperdicio de CPU/rede). Ademas no hay idempotency keys,
   asi que un retry del cliente puede duplicar side effects.

## Decisiones de diseno

### 1. Patron OpenCode: retry exponencial adaptativo
OpenCode (y agentes similares) usan el patron: `1s, 2s, 5s, 10s` con
jitter, donde cada operacion tiene su propio timeout. Adoptamos eso:
- **HTTP GET (listar conversaciones)**: timeout 5s, retry 3 veces
- **HTTP POST (enviar mensaje)**: timeout 10s, retry 2 veces
- **Polling**: timeout 3s, retry 5 veces, backoff adaptativo
- **Llamada al LLM**: timeout 60s, sin retry (el LLM no se reintenta
  porque ya consumio tokens; el usuario decide)

### 2. Persistencia local con IndexedDB (no localStorage)
Mensajes pendientes se guardan en IndexedDB (no localStorage) porque:
- Mayor capacidad (cuenta de GB vs 5MB)
- Estructurado (queries sobre estado)
- No bloquea el main thread

### 3. Three-state machine para cada operacion
Cada operacion del cliente puede estar en:
- `pending`: guardada localmente, no enviada al server
- `inflight`: enviada al server, esperando respuesta
- `confirmed`: server respondio OK, mensaje confirmado

### 4. Watchdog en backend para stuck messages
Un management command `python manage.py chat_watchdog` corre cada 5
minutos y marca como `error` los mensajes que llevan mas de 5 minutos
en `STATUS_PROCESSING`. Asi el cliente ve el error cuando vuelve.

### 5. Timeouts en 3 capas (defense in depth)
- **Cliente**: AbortController + timeout 10s en cada fetch
- **Admin (Django)**: `connect_timeout=5, statement_timeout=30` en OPTIONS de DB
- **MCP (Go)**: `context.WithTimeout` en cada handler

## Arquitectura (antes vs despues)

### ANTES
```
Cliente (fetch) -----> Admin (Django sync) -----> MCP (no ctx) -----> LLM
                       |                          |
                       v                          v
                    Postgres (no timeout)     MiniMax (timeout fijo 120s)
```

### DESPUES
```
Cliente (fetch + AbortController + retry + IndexedDB)
   |
   v
Admin (Django):
  - DB: connect_timeout=5, statement_timeout=30
  - LLM: timeout=60s
  - Async tasks via Celery (process_message en background)
  - Watchdog cada 5min marca stuck messages
   |
   v
MCP (Go):
  - context.WithTimeout(30s) en cada handler
  - Idempotency-Key header para deduplicar
   |
   v
LLM: MiniMax con timeout 60s
```

## Scope de cada HU

| HU | Alcance | Estimacion |
|----|---------|------------|
| 51.1 Client resilience | Offline detection, IndexedDB queue, exponential backoff, AbortController, reconnect UI | 2-3 dias |
| 51.2 Backend timeouts + watchdog | DB connect_timeout, statement_timeout, LLM timeouts por operacion, async tasks via Celery, watchdog management command | 3-4 dias |
| 51.3 MCP timeouts + idempotency | context.WithTimeout en handlers, idempotency keys, request cancellation, connection pool sizing | 2-3 dias |

**Total: 7-10 dias** distribuidos incrementalmente.

## Orden de implementacion recomendado

1. **HU-51.1 Client resilience** (mas urgente, afecta UX directo)
2. **HU-51.2 Backend timeouts + watchdog** (limpia el estado del server)
3. **HU-51.3 MCP timeouts + idempotency** (limpia el estado del MCP)

## Out of scope

- **No** se implementa offline-first completo (solo queue local con retry)
- **No** se migra el chat a WebSockets (HU-50.5 futuro)
- **No** se cambia el sistema de autenticacion (sigue con cookies Django)
- **No** se replica el state del cliente entre browsers/dispositivos

## Metricas de exito

| HU | Metrica | Target |
|----|---------|--------|
| 51.1 | Mensajes no perdidos durante desconexion | 0 perdidos |
| 51.1 | Tiempo de recovery despues de reconectar | < 5s |
| 51.2 | Stuck messages (en processing > 5min) | 0 |
| 51.2 | Requests que exceden statement_timeout | < 1% |
| 51.3 | Requests MCP cancelados por cliente desconectado | < 5% de desperdicio |
| 51.3 | Duplicate requests rechazados por idempotency key | 100% |

## Riesgos

| Riesgo | Mitigacion |
|--------|------------|
| IndexedDB no soportado en navegadores antiguos | Fallback a localStorage con limite |
| Watchdog mata messages legitimos | Threshold de 5min (no 1min) para dar margen |
| Statement timeout muy bajo mata queries largas | Configurar por operacion, default 30s |
| MCP cancela mid-LLM-call | LLM es idempotente en su mayoria; si no, registrar error y dejar que el cliente reintente |
| Idempotency key colision | UUIDv4 + scope por user_id |