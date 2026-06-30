# Inferencia de aristas de memoria — autónoma en el VPS

Job interno (system cron) que corre periódicamente la inferencia de aristas
tipadas entre memorias usando MiniMax-M3, SIN embeddings y SIN agente liberado.

## Qué hace

`EdgeInferencer` (`internal/scheduler/cron/system/edge_inference.go`):

1. Enumera los `project_id` con observations activas (`knowledge_observations`,
   `deleted_at IS NULL`), acotado a `ProjectBatch` por pasada.
2. Por cada proyecto llama a `observation.Service.InferEdgesLLM`, que:
   - arma pares candidatos por señales baratas (co-sesión, tags, solape léxico),
   - le pide a MiniMax-M3 que clasifique la relación de cada par,
   - crea las aristas resultantes con `origin='inferred'` (idempotente).

Su scope es EXACTAMENTE una operación: leer memorias + crear aristas. No tiene
acceso a tools, ni a un loop de razonamiento abierto. No es un cron user-defined
(tabla `crons`) ni un agente.

## Por qué system cron y no `domain_cron_create`

El cron user-defined dispatchea sólo `target_type ∈ {flow, agent, skill}`
(`internal/dispatch/dispatcher.go`):

- `agent` → daría un agente con razonamiento abierto y acceso a tools:
  exactamente el "agente liberado" que se quería evitar.
- `skill` → los tipos son `prompt|api|code|mcp_tool`; ninguno puede invocar la
  inferencia server-side (`code`/`mcp_tool` están sin implementar; `prompt`/`api`
  no llaman a `InferEdgesLLM`).
- `flow` → requeriría definir un flow + un step_type nuevo que envuelva la
  inferencia: mucha más superficie que el problema.

Por eso la inferencia autónoma vive como **system cron** (mismo patrón que
`HeartbeatWatcher` y `OrphanAuditor`): hardcoded, leader-gated, opt-in por flag.
La acción de inferencia SÍ queda además disponible on-demand vía la tool MCP
`domain_mem_infer_edges_llm` (para el caso "IDE conectado").

## Cómo se activa

Sólo corre en `cmd/domain` (el server HTTP con scheduler + leader election), no
en `cmd/domain-mcp` (stdio). Está deshabilitado por default (consume tokens):

| Env var | Default | Descripción |
| --- | --- | --- |
| `DOMAIN_EDGE_INFERENCE_ENABLED` | `false` | activa el job |
| `DOMAIN_EDGE_INFERENCE_TICK_HOURS` | `6` | cada cuántas horas corre una pasada |
| `DOMAIN_EDGE_INFERENCE_MAX_PAIRS` | `30` | pares candidatos por proyecto por pasada |
| `DOMAIN_EDGE_INFERENCE_PROJECT_BATCH` | `50` | proyectos por pasada (acota fan-out LLM) |
| `MINIMAX_API_KEY` | — | **requerido** para que el job corra (ver degradación) |

Ejemplo (`.env` del VPS):

```
MINIMAX_API_KEY=sk-...
DOMAIN_EDGE_INFERENCE_ENABLED=true
DOMAIN_EDGE_INFERENCE_TICK_HOURS=6
```

Tras reiniciar `cmd/domain`, el job arranca bajo el lock de leader (un solo nodo
del cluster lo corre) y dispara una pasada inmediata + cada `TICK_HOURS`.

## Degradación elegante (sin MINIMAX_API_KEY)

- Sin la key, el provider `minimax` no se registra. Al arrancar, el job hace un
  **precheck**: llama a `InferEdgesLLM` con un proyecto nulo. Si recibe
  `ErrInferenceUnavailable`, loguea `edge-inference disabled: MiniMax no
  configurado` y **no arranca el ticker** — sale limpio, no crashea el scheduler.
- Si MiniMax cae a mitad de una pasada, el error por proyecto se loguea y la
  pasada se aborta sin romper el resto del scheduler; la próxima pasada reintenta.
- El precheck no tiene efectos secundarios cuando MiniMax SÍ está: el proyecto
  nulo devuelve 0 candidatos sin llamar al LLM ni escribir aristas.

## Observabilidad

Logs estructurados (`slog`) con prefijo `edge-inference`:
`started`, `pasada completa` (projects/created/existing), `aristas creadas` por
proyecto, y warnings de degradación. Las aristas creadas quedan auditadas por
`EdgeService.Link` (origin `inferred`, `inferred_by=MiniMax-M3`).

## Archivos

- `internal/scheduler/cron/system/edge_inference.go` — el job.
- `internal/config/config.go` — flags `EdgeInference*`.
- `cmd/domain/server_runners.go` — wiring dentro del block `RunAsLeader`.
- `internal/service/observation/inference.go` — `InferEdgesLLM` (lógica reusada).
