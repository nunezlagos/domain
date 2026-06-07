# HU-08.4-multi-agent-orch

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** usuario avanzado del sistema de agentes
**Quiero** que un agente supervisor pueda delegar subtareas a agentes especializados, pasando contexto entre ellos, e incluso ejecutar múltiples agentes en paralelo
**Para** resolver problemas complejos que requieren diferentes perspectivas o capacidades

## Criterios de aceptación

### Scenario 1: Delegación supervisor → subagente
**Given** un agente supervisor "Architect" y un subagente "Code Reviewer"
**When** durante la ejecución, el supervisor solicita delegar al subagente con un contexto y pregunta específica
**Then** el subagente se ejecuta con el contexto recibido
**And** el resultado se retorna al supervisor
**And** el supervisor continúa su ejecución con el resultado

### Scenario 2: Handoff protocol (context passing)
**Given** un supervisor que delega al subagente "Bug Hunter"
**When** se pasa el contexto: `{files: ["src/main.go"], description: "revisar error de compilación"}`
**Then** el subagente recibe exactamente ese contexto
**And** el subagente ejecuta con su propio system prompt + modelo + skills
**And** el resultado incluye metadata del subagente (modelo usado, tokens, costo)

### Scenario 3: Ejecución paralela
**Given** un supervisor que identifica 3 subtareas independientes
**When** el orquestador las ejecuta en paralelo
**Then** las 3 ejecuciones ocurren concurrentemente
**And** el supervisor recibe los 3 resultados
**And** el tiempo total es ~max(tiempo individual), no la suma

### Scenario 4: Error en subagente
**Given** un subagente que falla durante la ejecución
**When** el supervisor espera el resultado
**Then** el error se captura y se pasa al supervisor como resultado de error
**And** el supervisor decide cómo continuar (reintentar, skip, fallar)

## Análisis breve

- **Qué pide realmente:** Un orquestador multi-agente donde un agente supervisor puede delegar a subagentes, con handoff de contexto, paralelismo y manejo de errores.
- **Módulos sospechados:** `internal/orch/`, `internal/agent/`, `internal/runner/`
- **Riesgos / dependencias:** Depende de agent execution (08.2), de agent definitions (08.1), de runs/logs (08.3), y del concepto de delegación que debe implementarse como un "skill" especial de orchestación.
- **Esfuerzo tentativo:** L**
