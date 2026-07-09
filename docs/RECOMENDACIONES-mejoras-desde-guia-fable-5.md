# Recomendaciones de mejora para domain MCP — derivadas de la Guía de Orquestación Fable 5

**Origen:** `~/Proyectos/fable-5/` (guía de orquestación y auto-gestión, issue-01.1
del proyecto `fable-5`). Cada recomendación mapea un patrón de la guía a una
mejora concreta del pipeline SDD de domain. La evidencia citada proviene de una
corrida real del orquestador (flow `a2056b29`, 2026-07-07) sobre el proyecto
`fable-5`.

**Alcance:** solo recomendaciones. No modifica código. Prioridad sugerida:
R1-R3 alta (afectan la corrección del round-trip), R4-R6 media (costo de
tokens/fricción), R7-R8 baja (ergonomía).

---

## R1 — Agregar protocolo de pivot al orquestador (Pilar 2 de la guía)

**Patrón de la guía:** el flujo serio no es lineal: necesita triggers de
replanificación, pausa de análisis (delta / coste-beneficio / preservación) y
return-to-work con estados `invalidada` + `reutiliza`.

**Estado actual:** el flow SDD es una secuencia fija (explore → spec → propose
→ design → tasks → apply → verify → judge → review → archive → onboard). Existe
`starting_phase` para reanudar, pero no hay: (a) estado `invalidated` para
steps/tasks, (b) registro del delta que motivó un replan, (c) forma de marcar
qué artefactos previos se reutilizan.

**Recomendación:** añadir a `flow_run_steps` un estado `invalidated` con campos
`reason` y `reuses[]`, y un tool `domain_orchestrate_pivot(flow_run_id, delta,
preserved_steps[], new_plan_notes)` que audite la pausa (los 3 pasos de la
guía) antes de permitir `starting_phase`. Hoy un replan honesto no deja rastro
auditable, que es exactamente lo que el SDD quiere evitar.

## R2 — Round-trip real de tasks: crear ids en BD desde sdd-tasks (Pilar 1, Plan de Vuelo)

**Patrón de la guía:** el Plan de Vuelo es la fuente de verdad y sus subtareas
conservan id a través de pivots y sesiones.

**Estado actual:** `parseTaskLine` (openspec/parse.go) solo sincroniza checkbox
de tasks con marcador `<!-- t:uuid -->`, y el uuid debe existir en BD. Pero la
fase sdd-tasks no crea tasks en BD ni devuelve ids: las tasks t1-t12 escritas
en `tasks.md` durante la corrida real NO tienen round-trip (el apply las ignora
silenciosamente y reporta `applied: [tasks.md]`, lo cual es engañoso).

**Recomendación:** que el handler de `sdd-tasks` en `domain_orchestrate_phase_result`
persista las tasks del output en `issue_tasks` y que el próximo
`domain_openspec_export` emita `tasks.md` con los marcadores `<!-- t:uuid -->`.
Además, que el apply reporte "tasks sin marcador: N ignoradas" en vez de
`applied` a secas.

## R3 — Alinear policy de specs con el parser real (Fase 4 de la guía: verificar contra la realidad)

**Patrón de la guía:** QA interna = comparar el artefacto contra el contrato
real, no contra el contrato documentado.

**Estado actual (bug de documentación):** la policy `openspec-spec-format` y el
system prompt de sdd-spec exigen `#### Scenario:` (4 hashtags) con líneas
`Given/When/Then` planas. El parser real (`ParseScenarios`, parse.go:51) exige
`## Scenario:` (2 hashtags) con bullets `- Given / - When / - Then`. En la
corrida real hicieron falta 3 intentos de apply para descubrir el formato
correcto leyendo el código fuente.

**Recomendación:** (a) alinear policy + prompts + parser en UN formato; (b) que
el error `spec.md no contiene escenarios válidos` incluya el formato esperado y
un ejemplo mínimo; (c) idealmente aceptar ambas variantes (H2/H4, con o sin
bullet) — el parseo es trivial de ampliar.

## R4 — Dieta de tokens en el plan del orquestador (Pilar 4: cada token paga su lugar)

**Estado actual:** `domain_orchestrate` devuelve el plan completo con los 11
system_prompts + user_prompts inline: ~385.000 caracteres en la corrida real.
Excede el límite de tool-result de los clientes LLM (el resultado terminó en un
archivo en disco) y repite el bloque completo de policies de plataforma en CADA
step (~20 policies × 11 steps).

**Recomendación:** devolver solo `flow_run_id` + primer step; los steps
siguientes ya llegan por `NextStepPrompt` en cada `phase_result` (mecanismo que
ya existe y funciona). Y en los system prompts, referenciar policies por slug
(`domain_policy_get` on demand) en vez de embeber los 20 cuerpos completos.
Ahorro estimado: >90% del payload inicial.

## R5 — Declarar `required_tool_calls` y el schema de output en el step (Pilar 3: contrato de delegación)

**Patrón de la guía:** todo encargo delegado lleva OBJETIVO + CONTEXTO + SALIDA
(schema exacto). El orquestador de domain delega fases al cliente LLM, pero el
contrato está incompleto.

**Estado actual:** (a) los steps del plan llegan con `required_tool_calls`
vacío y el server igual rechaza el cierre por `missing_tool_calls` (p. ej.
`domain_code_graph` en sdd-explore); (b) los campos requeridos del output se
descubren por goteo — sdd-spec rechazó primero por `issue_slug`, después por
`issue_md`, en llamadas sucesivas.

**Recomendación:** incluir en cada step del plan `required_tool_calls[]` y
`output_schema` (JSON Schema). Y que la validación devuelva TODOS los campos
faltantes en un solo `validation_error`, no de a uno.

## R6 — Pipeline adaptado a intent=doc (Pilar 1: fases condicionales)

**Patrón de la guía:** las fases son OBLIGATORIAS/CONDICIONALES según la
naturaleza de la tarea; forzar fases de código a una tarea de documentación
produce fricción sin valor.

**Estado actual:** con `mode=full`, una HU de documentación pura pasa por
prompts de TDD estricto, commits y sabotaje de tests pensados para Go. El
cliente tiene que reinterpretar ("TDD documental") cada fase.

**Recomendación:** que sdd-explore, al detectar `intent=doc`, sugiera un
pipeline `doc` (explore → spec → apply → verify) con prompts de verificación
documental (existencia, estructura, checks por grep), análogo al modo `lite`
que ya existe para fixes triviales.

## R7 — Mensajes de error accionables en openspec_apply

**Estado actual:** `issue_id inválido o falta .openspec.yaml` cuando el yaml
existe pero el issue no está en BD (el flujo correcto era crear el issue con el
wizard primero — eso no se deduce del mensaje). Además, los archivos NO
enviados en el apply se reportan como `conflicts` con "archivo vacío o borrado",
aunque simplemente se omitieron del array.

**Recomendación:** distinguir tres casos en la respuesta: `not_sent` (omitido
del array — no es conflicto), `unknown_issue` (con hint: "creá el issue con
domain_issue_create_* o corré export"), y `conflict` real (hash divergente).

## R8 — Bootstrap conversacional para proyectos nuevos (Fase 0: una sola ronda de preguntas)

**Patrón de la guía:** las ambigüedades se preguntan UNA vez, agrupadas.

**Estado actual:** para un proyecto nuevo, el flujo pide: cuestionario de
registro (5 datos) + wizard de issue de 8 preguntas + hardspec confirm +
posibles confirms por fase. Son hasta 4 interacciones separadas con el usuario
antes de escribir la primera línea.

**Recomendación:** permitir que `domain_session_register` y
`domain_issue_create_start` acepten todos los slots conocidos en la primera
llamada (batch), y que el wizard solo pregunte los que falten. El cliente LLM
ya suele tener esas respuestas tras su propia Fase 0.

---

## Resumen

| # | Mejora | Pilar de la guía | Prioridad |
|---|---|---|---|
| R1 | Protocolo de pivot auditable en flows | Pilar 2 | Alta |
| R2 | Round-trip de tasks con ids de BD | Pilar 1 | Alta |
| R3 | Alinear policy/prompt/parser de specs | Fase 4 | Alta |
| R4 | Plan del orquestador a dieta (payload −90%) | Pilar 4 | Media |
| R5 | Contrato completo por step (tools + schema) | Pilar 3 | Media |
| R6 | Pipeline `doc` para HUs de documentación | Pilar 1 | Media |
| R7 | Errores accionables en openspec_apply | — | Baja |
| R8 | Registro + wizard en batch | Fase 0 | Baja |
