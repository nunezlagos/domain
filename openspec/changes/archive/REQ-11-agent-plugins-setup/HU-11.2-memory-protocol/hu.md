# HU-11.2-memory-protocol

**Origen:** `REQ-11-agent-plugins-setup`
**Prioridad:** alta
**Tipo:** docs

## Historia de usuario

**Como** agente de IA (Claude, OpenCode, etc.)
**Quiero** un protocolo claro que defina cuándo y cómo usar la memoria
**Para** integrarme correctamente con engram sin comportamientos inconsistentes

**Como** desarrollador de agentes
**Quiero** reglas explícitas de: WHEN_TO_SAVE, WHEN_TO_SEARCH, topic update, session close, passive capture, after compaction
**Para** que todos los agentes sigan el mismo comportamiento

## Criterios de aceptación

```gherkin
Scenario: Protocol document define WHEN_TO_SAVE rules
  Given el protocol document
  When se lee la sección WHEN_TO_SAVE
  Then incluye: after completing a task, after learning project context, before closing session, when discovering decisions

Scenario: Protocol document define WHEN_TO_SEARCH rules
  Given el protocol document
  When se lee la sección WHEN_TO_SEARCH
  Then incluye: before making assumptions, when referencing past work, when detecting patterns

Scenario: Protocol document define topic update rules
  Given el protocol document
  When se lee la sección de topic update
  Then incluye: extract topic_key from content, reuse existing topics when similar, create new when distinct

Scenario: Protocol document define session close protocol
  Given el protocol document
  When se lee la sección de session close
  Then incluye: save final summary, call domain_mem_session_summary, set session status to closed

Scenario: Protocol document define passive capture rules
  Given el protocol document
  When se lee la sección de passive capture
  Then incluye: capture from tool output, capture errors and warnings, capture successful patterns

Scenario: Protocol document define after compaction behavior
  Given el protocol document
  When se lee la sección de after compaction
  Then incluye: re-search for consolidated topics, update references to merged observations

Scenario: Documento está en formato markdown accesible
  Given el protocol document
  When se verifica el formato
  Then es markdown con secciones numeradas
  And incluye ejemplos de código para cada regla

Scenario: Protocol es referenciable desde CLAUDE.md/AGENTS.md
  Given el setup crea CLAUDE.md
  When se configura claude-code
  Then CLAUDE.md incluye referencia al memory protocol

Scenario: Protocol versionado
  Given el protocol document
  When se verifica la metadata
  Then incluye version number y last updated date
```

## Análisis breve

- **Qué pide realmente:** Documento de protocolo para agentes con reglas explícitas de uso de memoria, formato markdown, versionado, referenciable desde configs de agentes
- **Módulos sospechados:** `docs/memory-protocol.md` o `internal/setup/protocol.go` (embedded)
- **Riesgos / dependencias:** Debe mantenerse actualizado con el código; el protocolo es una convención no enforced por código
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
- **Evidencia:** —
- **Acción derivada:** —
