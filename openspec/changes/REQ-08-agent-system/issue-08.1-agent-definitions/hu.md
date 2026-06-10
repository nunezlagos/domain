# issue-08.1-agent-definitions

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** usuario del sistema de agentes
**Quiero** crear, leer, actualizar y eliminar definiciones de agente con campos como nombre, slug, modelo, system_prompt, temperatura, max_tokens, skills asignados y project_id
**Para** poder definir agentes especializados con comportamientos y capacidades específicas

## Criterios de aceptación

### Scenario 1: Crear agente
**Given** un usuario autenticado con permisos de escritura
**When** envía una request POST /agents con body válido
**Then** el agente se crea con slug auto-generado desde name
**And** recibe un HTTP 201 con el agente creado
**And** el agente incluye un version id (v1)

### Scenario 2: Validación de campos requeridos
**Given** un payload sin campo `name` ni `model`
**When** se intenta crear el agente
**Then** se retorna HTTP 422
**And** el error lista los campos faltantes: name, model

### Scenario 3: Asignar skills
**Given** un agente existente
**When** se actualiza con `skills: ["skill-code-review", "skill-arch-advisor"]`
**Then** los skills se asocian al agente en la tabla `agent_skills`
**And** al consultar el agente, los skills aparecen en la respuesta

### Scenario 4: Slug único
**Given** un agente con slug "code-reviewer" ya existe
**When** se intenta crear otro con name "Code Reviewer"
**Then** el slug generado es "code-reviewer-2" o se retorna error de conflicto

### Scenario 5: Versionado
**Given** un agente que ya fue actualizado 3 veces
**When** se consulta el historial de versiones
**Then** retorna 3 versiones con sus campos en cada una
**And** la versión activa es la más reciente

## Análisis breve

- **Qué pide realmente:** CRUD completo de definiciones de agente con versionado, asignación de skills desde el skill registry, y validaciones de integridad.
- **Módulos sospechados:** `internal/agent/`, `internal/api/`, `internal/skill/`, `internal/model/`
- **Riesgos / dependencias:** Depende del model registry (issue-06.4) para validar modelo, del skill registry (issue-05.1, issue-05.2) para asignar skills, y del proyecto para scoping.
- **Esfuerzo tentativo:** L**
