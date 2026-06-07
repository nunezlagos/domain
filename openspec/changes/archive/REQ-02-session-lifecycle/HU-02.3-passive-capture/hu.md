# HU-02.3-passive-capture

**Origen:** `REQ-02-session-lifecycle`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** agente de IA procesando texto de una interacción
**Quiero** extraer automáticamente secciones "## Key Learnings:" con items numerados, bullets o checklists y guardarlos como observaciones individuales
**Para** que los aprendizajes importantes queden persistidos sin intervención manual del usuario

## Criterios de aceptación

```gherkin
Feature: Passive capture of learnings

  Scenario: Extraer sección Key Learnings con bullets
    Given el siguiente texto:
      """
      ## Key Learnings:
      - SQLite WAL mode permite lecturas concurrentes
      - UUID v7 es time-ordered y optimiza B-trees
      """
    When se llama a domain_mem_capture_passive con ese texto
    Then se crean 2 observaciones
    And la primera observación tiene content="SQLite WAL mode permite lecturas concurrentes"
    And la segunda observación tiene content="UUID v7 es time-ordered y optimiza B-trees"

  Scenario: Extraer sección Key Learnings con items numerados
    Given el siguiente texto:
      """
      ## Key Learnings:
      1. Usar transacciones para operaciones atómicas
      2. Validar entrada antes de escribir en DB
      """
    When se llama a domain_mem_capture_passive con ese texto
    Then se crean 2 observaciones
    And la primera tiene content="Usar transacciones para operaciones atómicas"

  Scenario: Extraer sección Key Learnings con checklist
    Given el siguiente texto:
      """
      ## Key Learnings:
      - [x] SessionStore implementado
      - [ ] Summary validation pendiente
      """
    When se llama a domain_mem_capture_passive con ese texto
    Then se crean 2 observaciones
    And el prefijo "- [x] " o "- [ ] " se limpia del content

  Scenario: Auto-skip duplicados por contenido exacto
    Given ya existe una observación con content="SQLite WAL mode permite lecturas concurrentes"
    And el texto contiene "## Key Learnings:\n- SQLite WAL mode permite lecturas concurrentes"
    When se llama a domain_mem_capture_passive con ese texto
    Then el item duplicado se salta silenciosamente
    And no se lanza error

  Scenario: Sin sección Key Learnings retorna error
    Given el texto no contiene "## Key Learnings:"
    When se llama a domain_mem_capture_passive con ese texto
    Then se retorna error "no learnings section found"

  Scenario: Key Learnings vacío no crea observaciones
    Given el texto contiene solo "## Key Learnings:" sin items debajo
    When se llama a domain_mem_capture_passive con ese texto
    Then se retorna error "learnings section is empty"

  Scenario: Items duplicados en mismo texto solo se guardan una vez
    Given el texto contiene items duplicados exactos
    When se llama a domain_mem_capture_passive con ese texto
    Then solo se crea una observación por cada item único

  Scenario: Múltiples secciones Key Learnings en un texto
    Given el texto tiene dos secciones "## Key Learnings:"
    When se llama a domain_mem_capture_passive
    Then se procesan ambas secciones
    And se crean todas las observaciones combinadas
```

## Análisis breve

- **Qué pide realmente:** Función `domain_mem_capture_passive(text, sessionID)` que parsea el texto buscando secciones `## Key Learnings:`, extrae items (bullets `-`, números `1.`, checklists `- [x]`), y guarda cada item como observación individual con dedup por contenido exacto; error si no hay sección
- **Módulos sospechados:** `internal/store/capture.go` — lógica de parseo + guardado
- **Riesgos / dependencias:** Depende de `ObservationStore` para guardar y buscar duplicados; el parseo debe ser robusto ante variaciones de markdown
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
- **Evidencia:** No existe `internal/store/capture.go`
- **Acción derivada:** Crear `internal/store/capture.go` con parser y extractor
