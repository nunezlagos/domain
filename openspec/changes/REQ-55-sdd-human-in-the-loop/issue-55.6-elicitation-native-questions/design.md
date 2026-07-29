# issue-55.6 elicitation-native-questions — Design

## Decisions
- D1. Transporte stateful: handler.go WithStateLess(false) + WithSessionIdleTTL(30m).
  Es el único requisito de infra; probado que Claude Code recibe elicitation sobre HTTP.
- D2. Capability: server.go NewMCPServer + WithElicitation(). Sin esto el cliente no
  sabe que el server puede pedir input.
- D3. API: srv.RequestElicitation(ctx, mcp.ElicitationRequest{Params:{Message, RequestedSchema}}).
  ctx debe traer la ClientSession (server.ClientSessionFromContext) — disponible en el
  handler de la tool. RequestedSchema = objeto plano con enum (limitación de la spec MCP:
  solo primitivos, no anidados; alcanza para opciones A/B/C + campo string texto libre).
- D4. Punto de disparo: la fase spec (sdd_spec.go / o el flujo de orchestrate). Cuando el
  agente detecta dudas, en vez de (solo) AskUserQuestion, el server puede pedir elicitation.
  OJO: el server no tiene LLM — la DECISIÓN de qué preguntar la formula el agente cliente;
  el server GARANTIZA que la pregunta se muestre y bloquee. Diseño exacto del punto de
  disparo se cierra en implementación (puede ser una tool domain_ask que el prompt de spec
  invoque, con el schema de la pregunta).
- D5. Fallback: si RequestElicitation devuelve ErrElicitationNotSupported (cliente sin
  soporte), caer al gate de evidencia (user_answers requerido en phase_result). Doble
  garantía: nativo donde se puede, bloqueo donde no.

## Alternatives
- A1. Solo gate de evidencia (sin elicitation): no fuerza UI, best-effort la presentación.
  Es el fallback, no la solución principal.
- A2. Refactor a stdio: elicitation nativa pero rompe el modelo HTTP/VPS. Descartado.

## Risk Mitigation
- WithSessionIdleTTL evita leak de sesiones zombie.
- Fallback gate cubre clientes sin elicitation (opencode, versiones viejas).
- Probe E2E ya de-riskeó el bloqueante principal (SSE del cliente).
