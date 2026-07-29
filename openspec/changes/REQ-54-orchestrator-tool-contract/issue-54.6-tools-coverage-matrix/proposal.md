# Como plataforma domain, cada una de las ~142 tools queda asignada a exactamente UN canal de invocación definido y auditable, con las 10 fases del SDD con contrato y prep poblados, de modo que "cubrir el 100% de las tools" sea un hecho verificable y no una aspiración.

## Why

REQ-54 construyó los MECANISMOS (contrato 54.1, prep 54.2, hooks lifecycle,
auto-trigger 54.4, planes de subagentes 54.5), pero la COBERTURA sigue parcial:
1 fase con contrato (sdd-verify), 3 con prep, y la mayoría de las tools sin
canal declarado. Sin una matriz explícita, "¿está cubierta la tool X?" no tiene
respuesta auditable, y las tools nuevas nacen huérfanas.

Definición honesta de 100%: **100% ASIGNADAS a un canal, no 100%
auto-invocadas.** Auto-invocar `domain_ticket_delete` o `domain_client_delete`
sería un bug, no una feature: esas son manuales POR DISEÑO y la matriz lo
documenta como decisión, no como omisión.

## Scope

**Canales (cada tool en exactamente uno):**

| Canal | Invocación | Estado |
|---|---|---|
| HOOK | determinista por evento del cliente (bootstrap, capture, turn) | hecho |
| FIRST-RESPONSE | protocolo de arranque de sesión | hecho |
| PHASE-CONTRACT | required_tool_calls de una fase SDD (server rechaza sin ellas) | 1/10 fases |
| PHASE-PREP | el server las corre y entrega el resultado (prep 54.2) | 3/10 fases |
| POLICY-TRIGGERED | normadas por policy (mem_save de decisiones, knowledge_save, lifecycle de skills/policies) | informal |
| USER-INTENT | manuales por diseño (CRUD, deletes, admin, crons, clients) | sin documentar |

**Entregables:**
1. Matriz completa tool→canal como knowledge doc en BD (fuente canónica,
   consultable vía domain_knowledge_search) + primer local.
2. Seeds de `required_tool_calls` y mapeo prep para las 10 fases del SDD
   (activación completa de 54.1 y 54.2).
3. Validación anti-regresión: chequeo (test Go contra el registry de tools)
   que falla si una tool del server no aparece en la matriz — las tools
   nuevas DEBEN declarar canal para pasar CI.

## Approach

Primero la matriz (clasificación auditada de las ~142), después los seeds por
fase (qué contrato y qué prep le corresponde a cada una de las 10), al final
el test anti-regresión que congela la invariante "cero huérfanas".

## Risks

- Clasificación discutible en tools ambiguas (ej: domain_code_build) → la
  matriz registra el criterio por tool; cambiarla es editar el doc, no código.
- El test anti-regresión necesita leer la lista real de tools del server →
  usar el registry de server.Tools() como fuente, no una lista copiada.

## Testing

El propio test anti-regresión es el corazón: registry ⊆ matriz. Más tests de
seeds (cada fase con contrato/prep esperados tras el seed).
