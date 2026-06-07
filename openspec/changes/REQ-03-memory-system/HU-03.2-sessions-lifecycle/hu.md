# HU-03.2-sessions-lifecycle

**Origen:** `REQ-03-memory-system`
**Persona:** dx-engineer, org-member
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** agente de IA operando en el sistema de memoria
**Quiero** iniciar y finalizar sesiones, consultar la sesión activa y obtener el resumen de sesiones pasadas
**Para** mantener contexto de la conversación actual y consultar el historial de interacciones previas

## Criterios de aceptación

```gherkin
Feature: Session Lifecycle Management

  Background:
    Given existe la tabla sessions en Postgres

  Scenario: Iniciar una sesión nueva
    When inicio una sesión con:
      | id         | "sesion-abc-123"        |
      | user_id    | "user-juan"             |
      | project_id | "proj-opencode-core"    |
      | directory  | "/home/user/project"    |
    Then se crea un registro en sessions con started_at = now()
    And el estado de la sesión es "active"
    And no hay ended_at (NULL)

  Scenario: Obtener sesión activa
    Given existe una sesión activa con id "sesion-abc-123"
    When consulto la sesión activa para el proyecto "opencode-core"
    Then obtengo la sesión "sesion-abc-123"
    And su estado es "active"
    And su started_at no es NULL

  Scenario: Finalizar una sesión activa
    Given existe una sesión activa con id "sesion-abc-123"
    When finalizo la sesión con summary "Se completaron 3 tareas"
    Then ended_at se setea al timestamp actual
    And el summary se guarda como "Se completaron 3 tareas"
    And la sesión ya no aparece como activa

  Scenario: Consultar estado de sesión
    Given existe una sesión activa "sesion-abc-123"
    And una sesión finalizada "sesion-def-456"
    When consulto el estado de "sesion-abc-123"
    Then obtengo status = "active"
    When consulto el estado de "sesion-def-456"
    Then obtengo status = "completed"

  Scenario: No se puede iniciar sesión duplicada
    When intento iniciar una sesión con id ya existente
    Then recibo un error de UniqueViolation
    And la sesión original no se modifica

  Scenario: No se puede finalizar una sesión ya finalizada
    Given existe una sesión finalizada "sesion-def-456"
    When intento finalizarla nuevamente
    Then recibo un error indicando que la sesión ya está cerrada
```

## Análisis breve

- **Qué pide realmente:** Tabla `sessions` con CRUD básico (start, end, get active, get by id, list por project). Active session tracking mediante flag o presencia de ended_at.
- **Módulos sospechados:** `internal/store/pg/session.go`, `internal/memory/session.go`
- **Riesgos / dependencias:** Bajo. Tabla simple, sin dependencias de otras HUs.
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
