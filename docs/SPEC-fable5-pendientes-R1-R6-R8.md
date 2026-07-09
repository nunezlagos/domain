# SPEC propuesta — pendientes de la guía fable-5 (R1, R6, R8)

**Estado:** propuesta (spec, sin implementar).
**Fecha:** 2026-07-09.
**Origen:** los 3 ítems de baja prioridad de `docs/RECOMENDACIONES-mejoras-desde-guia-fable-5.md` que quedaron sin abordar tras implementar R2/R3/R4/R5/R7. Verificados contra el código actual durante la sesión.
**Alcance:** solo diseño. Cada uno sería un flow SDD `full` independiente cuando se priorice.

Contexto: R2, R3, R4, R5, R7 ya están implementados y en `main` (issues 64.1/64.2/64.3 + 54.1 + fix started_at). Estos 3 son los de menor ratio impacto/costo y por eso quedaron para el final.

---

## R1 — Protocolo de pivot auditable en flows

### Problema (verificado)
El flow SDD es una secuencia fija. Los estados de step son `pending/running/completed/failed/paused/cancelled/skipped` (`internal/service/flow/state_machine.go:38-44`). **No existe** un estado `invalidated` ni forma de registrar que un replan reutiliza artefactos previos. Cuando el trabajo cambia de rumbo a mitad de un flow (el spec resultó incompleto, apareció un requisito nuevo), hoy la única salida es `cancelled` + re-orquestar — y se pierde el rastro de POR QUÉ se replanificó y QUÉ se reutiliza. Es exactamente lo que el SDD busca evitar: un cambio de rumbo sin auditoría.

### Propuesta
Agregar un estado `invalidated` a `flow_run_steps` + un tool `domain_orchestrate_pivot` que audite la pausa de replanificación (los 3 pasos de la guía: delta / coste-beneficio / preservación) antes de permitir `starting_phase`.

### Requisitos (borrador)

#### Requirement: estado invalidated con razón y reúso
Un `flow_run_step` MUST poder transicionar a `invalidated` con un campo `reason` (por qué se invalidó) y `reuses` (lista de step IDs cuyos artefactos se conservan). `invalidated` es terminal-para-ese-intento pero NO cancela el flow_run.

- **Given** un flow con steps ya completados y un cambio de rumbo del usuario
- **When** se invoca `domain_orchestrate_pivot(flow_run_id, delta, preserved_steps[], reason)`
- **Then** los steps afectados pasan a `invalidated` con su reason, los `preserved_steps` conservan su estado, y el flow queda listo para re-planificar desde `starting_phase`

#### Requirement: el pivot deja rastro auditable
El pivot MUST persistir el delta que lo motivó (mem_save type=decision o tabla de audit), de modo que el historial del flow muestre "se replanificó en la fase X por el motivo Y, reutilizando Z".

### Riesgos / decisiones abiertas
- ¿`invalidated` es un estado nuevo en el CHECK de `flow_run_steps.status` (migración) o un flag aparte? Recomendación: estado nuevo (más explícito en queries de audit).
- ¿El pivot lo dispara solo el usuario, o también el orquestador al detectar un spec inconsistente? Recomendación: solo usuario al inicio (YAGNI); auto-pivot es v2.
- Interacción con la máquina de estados (`stepTransitions`): agregar las transiciones válidas hacia `invalidated`.

### Esfuerzo estimado
**M** (medium). Toca: migración (CHECK), `state_machine.go` (transiciones), tool nuevo, tabla/mem de audit. Es el más grande de los 3.

---

## R6 — Pipeline `doc` para HUs de documentación

### Problema (verificado)
Los modos del orquestador son `async/lite/express/solo/full` (`internal/service/orchestrator/modes/`, verificado: no hay `doc.go`). Una HU de documentación pura (escribir/actualizar docs, sin código) pasa por prompts de TDD estricto pensados para Go — el cliente tiene que reinterpretar cada fase como "TDD documental", con fricción y sin valor. `lite` ya absorbe parte del dolor (subset de fases) pero sigue asumiendo cambio de código.

### Propuesta
Un modo `doc` (o un `intent=doc` detectado en `sdd-explore`) que arme un pipeline `explore → spec → apply → verify` con prompts de verificación DOCUMENTAL (existencia de secciones, estructura, checks por grep) en vez de TDD de código. Análogo a cómo `lite` ya existe para fixes triviales.

### Requisitos (borrador)

#### Requirement: pipeline doc sin fases de código
Cuando el intent es documentación, el orquestador MUST ofrecer un pipeline reducido (`explore → spec → apply → verify`) SIN las fases `judge`/sabotaje de código, y con un prompt de `verify` que valide el artefacto documental (existe, tiene las secciones esperadas, links no rotos) en vez de correr tests.

- **Given** una HU cuyo intent es documentación pura
- **When** el orquestador arma el plan
- **Then** el plan usa el pipeline doc (verificación documental) y no inyecta prompts de TDD/sabotaje de Go

#### Requirement: detección de intent doc
`sdd-explore` SHOULD detectar `intent=doc` (el pedido menciona docs/README/CHANGELOG/spec y NO toca archivos de código) y sugerir el modo doc; el usuario confirma.

### Riesgos / decisiones abiertas
- ¿Modo explícito (`mode=doc`) o intent auto-detectado? Recomendación: ambos — auto-detección que propone, override manual.
- La fase `verify` documental necesita su propio prompt seed. OJO: es un seed → requiere bumpear `Version()` del seeder para propagar (gotcha verificado esta sesión).

### Esfuerzo estimado
**S-M**. Reusa la infra de modes; el grueso es el nuevo `doc.go` + los prompts seed de verificación documental.

---

## R8 — Bootstrap conversacional en batch para proyectos nuevos

### Problema (verificado)
Para un proyecto nuevo, el flujo pide en interacciones separadas: cuestionario de registro (5 datos) + wizard de issue de 8 preguntas + hardspec confirm + posibles confirms por fase. Son hasta 4 rondas de ida y vuelta antes de escribir la primera línea. El cliente LLM ya suele tener esas respuestas tras su propia Fase 0, pero el protocolo las pide de a una.

### Propuesta
Permitir que `domain_session_register` y `domain_issue_create_start` acepten TODOS los slots conocidos en la primera llamada (batch), y que el wizard solo pregunte los que falten. La guía fable-5: "las ambigüedades se preguntan UNA vez, agrupadas".

### Requisitos (borrador)

#### Requirement: register acepta slots en batch
`domain_session_register` MUST aceptar los datos del proyecto (nombre, workflow, estructura, etc.) en una sola llamada si el cliente los provee, sin forzar un ida-y-vuelta por cada campo.

- **Given** un cliente que ya tiene los datos del proyecto tras su Fase 0
- **When** llama `domain_session_register` con todos los slots
- **Then** el proyecto queda registrado sin preguntas adicionales

#### Requirement: el wizard de issue solo pregunta lo faltante
`domain_issue_create_start` MUST aceptar slots pre-poblados (title, change_type, etc.) y el wizard SOLO MUST preguntar los que no vinieron, en vez de las 8 preguntas siempre.

- **Given** un `issue_create_start` con varios slots ya provistos
- **When** arranca el wizard
- **Then** solo pregunta los slots faltantes (no re-pregunta los provistos)

### Riesgos / decisiones abiertas
- Compatibilidad: los slots batch deben ser ADITIVOS (opcionales) para no romper el flujo pregunta-a-pregunta actual.
- Conecta con R5 (contrato upfront): si el cliente sabe qué slots existen de antemano, puede pre-poblarlos. R5 ya expone parte de eso.

### Esfuerzo estimado
**S**. Campos opcionales aditivos en 2 tools + lógica "preguntar solo lo faltante" en el wizard.

---

## Resumen y recomendación de orden

| # | Mejora | Pilar guía | Esfuerzo | Impacto | Orden sugerido |
|---|--------|-----------|----------|---------|----------------|
| R8 | Bootstrap batch | Fase 0 | S | Medio (solo proyectos nuevos) | 1º (más barato) |
| R6 | Pipeline doc | Pilar 1 | S-M | Medio (fricción en HUs de doc) | 2º |
| R1 | Pivot auditable | Pilar 2 | M | Alto si hay replanes frecuentes | 3º (el más grande) |

**Recomendación:** ninguno es urgente (por eso quedaron al final). Si se retoman, arrancar por **R8** (más barato, campos aditivos) y **R6** (reusa infra de modes). **R1** es el más valioso conceptualmente pero también el más caro — conviene cuando haya evidencia de replanes frecuentes que justifiquen la máquina de estados nueva.

Cada uno debería ir por su propio flow SDD `full` cuando se priorice. Al tocar seeds de prompts (R6), recordar bumpear el `Version()` del seeder.
