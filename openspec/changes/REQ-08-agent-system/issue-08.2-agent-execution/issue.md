# issue-08.2-agent-execution

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** usuario del sistema
**Quiero** ejecutar un agente con un input, y que el sistema cree un run record, inicialice el contexto (system prompt + memories + skills), invoque al LLM y maneje skill invocations intermedias
**Para** obtener respuestas del agente con su configuración completa y capacidades asociadas

## Criterios de aceptación

### Scenario 1: Ejecución básica
**Given** un agente "Code Reviewer" configurado con modelo gpt-4, temp=0.3, max_tokens=2000
**When** se envía una request POST /agents/code-reviewer/run con input "Revisa este PR #42"
**Then** se crea un run record con status=running
**And** el sistema carga el system prompt del agente
**And** el sistema carga los skills asignados al agente
**And** se invoca al LLM con system prompt + input + skill registry
**And** se retorna el output del LLM
**And** el run se marca como completed

### Scenario 2: Skill invocation durante ejecución
**Given** un agente con skill "list-files" asignado
**When** durante la ejecución el LLM solicita ejecutar el skill "list-files" con args `{path: "./src"}`
**Then** el sistema ejecuta el skill
**And** alimenta el resultado de vuelta al LLM como tool response
**And** el LLM continúa la generación con el resultado

### Scenario 3: Error durante ejecución
**Given** un agente con un modelo inválido (no disponible)
**When** se intenta ejecutar el agente
**Then** el run se crea con status=failed
**And** el error se registra: "model_not_available"
**And** no se consume presupuesto de tokens

### Scenario 4: Ejecución con contexto enriquecido
**Given** un agente que tiene memories y knowledge asociados
**When** se ejecuta el agente
**Then** el contexto incluye: system prompt + memories relevantes + knowledge docs + skills registry

## Análisis breve

- **Qué pide realmente:** Un motor de ejecución de agentes que orqueste: creación de run, carga de contexto, invocación al LLM con tool/skill handling, y retorno de output con log completo.
- **Módulos sospechados:** `internal/agent/`, `internal/runner/`, `internal/llm/`, `internal/skill/`, `internal/context/`
- **Riesgos / dependencias:** Depende de agent definitions (08.1), skill execution (05.5), LLM provider (06.1), memory system (REQ-03), context optimizer (07.1), token budget (07.4).
- **Esfuerzo tentativo:** XL**
