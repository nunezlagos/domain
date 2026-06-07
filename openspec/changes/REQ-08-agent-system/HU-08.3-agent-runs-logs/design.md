# Design: HU-08.3-agent-runs-logs

## Decisión arquitectónica

**Patrón:** Append-only event log con dos niveles: run summary (cabecera) + event details (detalle).

```
AgentExecutor ──▶ RunLogger ──▶ agent_runs (summary)
                     │              │
                     ├──▶ RunCreated event
                     ├──▶ LLMCall event
                     ├──▶ SkillExecution event
                     ├──▶ RunCompleted event
                     └──▶ RunFailed event
                              │
                              ▼
                         run_logs (event stream)
```

## Alternativas descartadas

1. **JSONB único con todos los eventos:** Dificulta query de eventos específicos (ej: "todos los skills que fallaron"). Preferimos filas por evento.
2. **Logs en archivos externos (archivos JSON):** Más difíciles de correlacionar con runs en DB. DB es suficiente para MVP.
3. **Solo summary sin detalle:** No permite debuggear tool calls. El detalle es esencial.

## Diagrama

```
Tablas:
┌─────────────────────────┐     ┌────────────────────────────┐
│       agent_runs        │     │        run_logs            │
├─────────────────────────┤     ├────────────────────────────┤
│ id (PK)                 │     │ id (PK)                    │
│ agent_id (FK)           │     │ run_id (FK)                │
│ project_id (FK)         │     │ type: llm_call|skill_exec  │
│ status: running|compltd │     │ sequence (order within run)│
│ input (TEXT)            │     │ timestamp                  │
│ output (TEXT, nullable) │     │ prompt (TEXT, nullable)    │
│ tokens_input            │     │ response (TEXT, nullable)  │
│ tokens_output           │     │ tool_calls (JSONB)         │
│ cost_input              │     │ tool_responses (JSONB)     │
│ cost_output             │     │ skill_name (nullable)      │
│ model_id                │     │ args (JSONB, nullable)     │
│ started_at              │     │ result (TEXT, nullable)    │
│ ended_at                │     │ duration_ms                │
│ error (TEXT, nullable)  │     │ tokens_used                │
│ created_at              │     │ status: success|failed     │
└─────────────────────────┘     └────────────────────────────┘
```

## TDD plan

1. **Red:** Test crear run + agregar logs → consultar GET /runs/:id retorna run con datos
2. **Green:** Implementar RunRepository + RunLogger básico
3. **Refactor:** Agregar cálculo de costo, paginación, filtros
4. **Sabotaje:** Run sin logs → GET /runs/:id/log retorna array vacío, no null

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| Prompts grandes saturan DB | Límite de 10KB en prompt/response; truncar con marcador si excede |
| Costo del modelo cambia post-ejecución | Snapshot de costos en agent_runs al momento de finalizar |
| Muchos logs = DB pesada | Indexar por run_id + sequence; política de retención (30 días por defecto) |
| Logging síncrono ralentiza ejecución | Logging asíncrono con buffer; si falla escritura, no afecta ejecución
