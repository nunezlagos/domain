# Hallazgo: MCP Elicitation sobre HTTP — VIABLE (probado E2E)

**Fecha:** 2026-07-02. **Contexto:** cómo lograr que el SDD "jamás se responda solo ante dudas".
(Este archivo es respaldo local — el VPS estaba caído al momento de guardar en domain_mem_save; migrar a memoria cuando vuelva.)

## Veredicto: elicitation funciona sobre HTTP con la lib actual de domain (mark3labs/mcp-go v0.54.1)

Probado con un probe end-to-end (server + cliente, misma lib): round-trip completo.
```
[server] tool llamada; sesión soporta elicitation=true
[cliente] recibió elicitation: ¿Opción A o B?
RESULTADO: elicit OK: action=accept content=map[choice:A]
```

## Condiciones (las DOS necesarias)
1. **Server**: `WithStateLess(false)` (stateful) + `WithElicitation()` capability.
2. **Cliente**: `WithContinuousListening()` — mantiene el canal SSE abierto.
   Sin esto → `context deadline exceeded` en el POST del tool (fue el primer fallo).

## Bloqueante restante: RESUELTO ✅ (probado con el cliente REAL de Claude Code)
Test end-to-end (2026-07-02 21:12): server MCP stateful local + `claude -p` headless
(Claude Code 2.1.191) apuntado al server + hook `Elicitation` que auto-responde.
Resultado triple-confirmado:
- server: `sesión soporta elicitation=true` → `elicitation OK: action=accept content=map[choice:A]`
- hook de Claude Code: `ELICITATION_RECIBIDA_POR_CLAUDE_CODE` (mismo timestamp)
- cliente devolvió `ELICIT_OK action=accept content=map[choice:A]`
CONCLUSIÓN: el cliente de Claude Code SÍ mantiene el canal SSE y recibe elicitation
sobre HTTP. NO hace falta que el cliente configure continuous listening — Claude Code
lo maneja nativo. El único requisito es del lado SERVER (stateful + WithElicitation).

## Costo del refactor en domain (bajo)
- `internal/mcp/httpserver/handler.go:~101`: `WithStateLess(true)` → `WithStateful(true)` + `WithSessionIdleTTL(30*time.Minute)` (evitar leak).
- `internal/mcp/server/server.go:~35`: agregar `WithElicitation()`.
- Hooks stateless (curl POST) sobreviven; no aprovechan elicitation salvo que mantengan session_id.

## Concurrencia (escenario 5 usuarios) — OK
Aislamiento por `Mcp-Session-Id` único + `Principal` (API key) + RLS en Postgres.
Los 2 usuarios en el mismo proyecto = 2 sesiones distintas, sin cross-talk.
Caddy 1 backend: sin cambios. N backends: necesitaría sticky sessions.

## Alternativa sin refactor (elegida antes por el usuario, sigue válida)
**Regla + gate de evidencia real**: sdd-spec exige `user_answers` en el phase_result;
sin respuestas del humano → server rechaza el cierre → el flujo se detiene. 100% de
"no se responde solo" sobre stateless, ~1h, cero riesgo. La UI de la pregunta queda
best-effort (AskUserQuestion), pero el BLOQUEO es duro.

## Decisión pendiente
- Ruta A: elicitation (UI nativa garantizada) — requiere verificar SSE de Claude Code + refactor stateful.
- Ruta B: regla+gate evidencia (bloqueo duro, sin UI nativa) — implementable ya, cero riesgo.
Ambas garantizan "jamás se responde solo". Difieren en la UI de la pregunta y el costo/riesgo.
