# Como usuario del SDD, en las fases spec/design el servidor FUERZA la pregunta con un diálogo nativo (MCP elicitation) imposible de saltear, en vez del AskUserQuestion best-effort que el modelo puede ignorar — garantizando que jamás se responda solo ante dudas.

## Why
El requisito del usuario: "que jamás se responda solo ante una formulación de dudas".
AskUserQuestion lo decide el modelo (best-effort, verificado contra doc oficial) → se
saltea. El gate server-side bloquea el avance pero no fuerza la UI de pregunta.
MCP elicitation SÍ: el server origina la pregunta, el cliente la muestra y bloquea
hasta responder.

## Evidencia (probado, no teoría)
Test E2E 2026-07-02 con el cliente REAL de Claude Code 2.1.191:
- server MCP stateful (mark3labs/mcp-go v0.54.1) + WithElicitation + tool que dispara
  srv.RequestElicitation → `claude -p` headless + hook Elicitation auto-respondedor.
- Triple confirmación: server logueó `elicitation OK action=accept`; hook de Claude Code
  disparó (`ELICITATION_RECIBIDA`); cliente devolvió `ELICIT_OK content=choice:A`.
- Conclusión: Claude Code mantiene el canal SSE nativo; elicitation funciona sobre HTTP
  sin config extra del cliente. Único requisito: SERVER stateful + capability.

## Scope
- Server stateful: handler.go WithStateLess(true)→false + WithSessionIdleTTL(30m).
- Capability: server.go WithElicitation().
- Fase spec (y design): cuando hay dudas, disparar srv.RequestElicitation con schema
  enum (opciones) — el usuario elige o escribe (texto libre vía campo string).
- Fallback: si la sesión NO soporta elicitation (ErrElicitationNotSupported), caer al
  gate de evidencia (user_answers en phase_result bloquea el cierre).

## Concurrencia (verificado)
5 usuarios simultáneos aislados por Mcp-Session-Id único + Principal (API key) + RLS.
Los 2 en el mismo proyecto = 2 sesiones distintas, sin cross-talk. Caddy 1 backend: OK.

## Risks
- Memory leak sin TTL → mitigado con WithSessionIdleTTL(30m).
- Hooks stateless (curl POST sueltos) sobreviven pero no aprovechan elicitation (no la
  necesitan; su trabajo es lifecycle, no preguntas).
- Caddy N backends (futuro) → necesitaría sticky sessions. Hoy 1 backend, no aplica.

## Testing
Probe E2E ya validó el transporte. Tests Go del handler de elicitation en la fase spec
(session soporta → dispara; no soporta → fallback gate).
