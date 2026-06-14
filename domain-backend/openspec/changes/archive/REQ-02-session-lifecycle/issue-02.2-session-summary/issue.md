# issue-02.2-session-summary

**Origen:** `REQ-02-session-lifecycle`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** agente de IA cerrando una sesión de trabajo
**Quiero** guardar un resumen estructurado con Goal, Instructions, Discoveries, Accomplished, Next Steps y Relevant Files
**Para** que la próxima vez que trabaje en este proyecto pueda retomar exactamente donde lo dejé sin perder contexto

## Criterios de aceptación

```gherkin
Feature: Session summary

  Scenario: Guardar resumen completo con todos los campos
    Given una sesión activa con id "sess-123"
    When se llama a domain_mem_session_summary con:
      | field           | value                              |
      | Goal            | Implementar sessions CRUD          |
      | Instructions    | Usar UUID v7 como session ID       |
      | Discoveries     | modernc.org/sqlite soporta FTS5    |
      | Accomplished    | SessionStore creado con Start/End  |
      | Next Steps      | Agregar resumen estructurado       |
      | Relevant Files  | internal/store/session.go          |
    Then el resumen se guarda asociado a la sesión "sess-123"
    And cada campo se almacena correctamente

  Scenario: Recuperar resumen por session id
    Given una sesión "sess-123" con resumen guardado
    When se consulta mem_session_summary_get("sess-123")
    Then se retorna el resumen completo con todos los campos

  Scenario: Resumen con campos faltantes da error de validación
    Given una sesión activa con id "sess-123"
    When se llama a domain_mem_session_summary con Goal vacío y Accomplished vacío
    Then se retorna error "summary validation failed: Goal and Accomplished are required"

  Scenario: Resumen excede tamaño máximo
    Given una sesión activa con id "sess-123"
    When se llama a domain_mem_session_summary con un campo de más de 10000 caracteres
    Then se retorna error "summary field exceeds maximum length"

  Scenario: Resumen requiere sesión activa
    Given una sesión "sess-123" que ya está "completed"
    When se llama a domain_mem_session_summary con campos válidos
    Then se retorna error "cannot add summary to completed session"

  Scenario: Resumen opcional al cerrar sesión (desde issue-02.1)
    Given una sesión activa con id "sess-123"
    When se llama a domain_mem_session_end con id="sess-123" y summary="resumen inline"
    Then el resumen inline se guarda
    But no se requiere estructura de campos

  Scenario: Sobrescribir resumen existente
    Given una sesión "sess-123" con resumen previo
    When se llama a domain_mem_session_summary con nuevos valores
    Then el resumen se actualiza
    And updated_at se refresca
```

## Análisis breve

- **Qué pide realmente:** Función `domain_mem_session_summary(session_id, Summary{Goal, Instructions, Discoveries, Accomplished, NextSteps, RelevantFiles})` con validación de campos requeridos (Goal, Accomplished), límite de tamaño por campo, y asociación a la sesión; función `mem_session_summary_get(session_id)` para recuperarlo
- **Módulos sospechados:** `internal/store/session.go` — extender SessionStore con SetSummary/GetSummary; posible nuevo archivo `internal/store/summary.go` si la lógica crece
- **Riesgos / dependencias:** El summary se almacena como JSON en la columna `summary` de `sessions`; si se decide struct separado, puede ir a tabla `session_summaries`
- **Esfuerzo tentativo:** M

## Verificación previa

- [x] Revisar codebase (grep) — proyecto greenfield
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe código de summary
- **Acción derivada:** Extender `SessionStore` con métodos Summary
