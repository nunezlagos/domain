# HU-02.1-session-start-end

**Origen:** `REQ-02-session-lifecycle`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** agente de IA trabajando en una sesión con el usuario
**Quiero** registrar el inicio y fin de cada sesión con id, proyecto, directorio y timestamps
**Para** poder trackear qué pasó en cada interacción, mostrar el estado de la sesión activa y cerrar sesiones correctamente

## Criterios de aceptación

```gherkin
Feature: Session start and end

  Scenario: Iniciar sesión con datos válidos
    Given un proyecto "Domain" en directorio "/home/user/memoria"
    When se llama a domain_mem_session_start con project="Domain" y directory="/home/user/memoria"
    Then se crea una sesión con id único no vacío
    And la sesión tiene project="Domain"
    And la sesión tiene directory="/home/user/memoria"
    And started_at no es nulo
    And status es "active"
    And ended_at es nulo

  Scenario: Cerrar sesión activa
    Given una sesión activa con id "sess-123"
    When se llama a domain_mem_session_end con id="sess-123"
    Then ended_at se establece con la fecha actual
    And status cambia a "completed"

  Scenario: Cerrar sesión con resumen opcional
    Given una sesión activa con id "sess-123"
    When se llama a domain_mem_session_end con id="sess-123" y summary="Trabajamos en REQ-02"
    Then ended_at se establece
    And status es "completed"
    And summary es "Trabajamos en REQ-02"

  Scenario: Estado inicial de sesión es "active"
    Given una sesión recién creada
    When se consulta el campo status
    Then debe ser "active"

  Scenario: Badge de sesión activa en UI
    Given una sesión activa con id "sess-123"
    When se renderiza el badge de estado
    Then debe mostrar un indicador verde con texto "active"

  Scenario: Error al cerrar sesión desconocida
    Given no existe una sesión con id "sess-999"
    When se llama a domain_mem_session_end con id="sess-999"
    Then se retorna error "session not found"

  Scenario: Error al cerrar sesión ya finalizada
    Given una sesión con id "sess-123" que ya tiene status "completed"
    When se llama a domain_mem_session_end con id="sess-123"
    Then se retorna error "session already ended"

  Scenario: Consultar estado de sesión por id
    Given una sesión activa con id "sess-123"
    When se consulta mem_session_status con id="sess-123"
    Then se retorna status="active", started_at, ended_at=nil
```

## Análisis breve

- **Qué pide realmente:** Funciones `domain_mem_session_start(id, project, directory)`, `domain_mem_session_end(id, summary?)`, `mem_session_status(id)` con persistencia en tabla `sessions`; manejo de errores para session not found y already ended; badge visual en UI (TUI) que muestre sesión activa
- **Módulos sospechados:** `internal/store/session.go` para store layer; `internal/tui/` para badge de estado
- **Riesgos / dependencias:** Depende de tabla `sessions` existente (HU-01.1); concurrencia: dos agentes iniciando sesión simultáneamente podrían generar IDs duplicados si no se usa UUID
- **Esfuerzo tentativo:** M

## Verificación previa

- [x] Revisar codebase (grep) — proyecto greenfield, sin Go code aún
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe `internal/store/session.go`
- **Acción derivada:** Crear `internal/store/session.go` con SessionStore, implementar start/end/status
