# HU-03.3-prompts-storage

**Origen:** `REQ-03-memory-system`
**Persona:** dx-engineer, org-member
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** agente de IA
**Quiero** almacenar y buscar prompts de usuario con full-text search, con un buffer process-local para captura inmediata en domain_mem_save
**Para** mantener historial de interacciones y poder reconsultar prompts previos relevantes

## Criterios de aceptación

```gherkin
Feature: Prompt Storage with FTS and Process-Local Buffer

  Background:
    Given existe la tabla prompts en Postgres con tsvector

  Scenario: Guardar un prompt
    When guardo un prompt con content "¿Cómo implemento un GIN index en Postgres?"
    Then se persiste con id único
    And created_at se setea al timestamp actual
    And se genera tsvector automáticamente

  Scenario: Buscar prompts por contenido
    Given existen prompts con contenido variado
    When busco prompts con la query "GIN index"
    Then obtengo prompts rankeados por relevancia
    And incluyen headline con el fragmento destacado

  Scenario: Buffer process-local para captura inmediata
    When ejecuto domain_mem_save con capture_prompt = true
    Then el prompt actual se captura en un buffer en memoria
    And se persiste asincrónicamente a Postgres
    And la respuesta no espera a la escritura en DB

  Scenario: Recuperar prompts por sesión
    Given prompts guardados asociados a una sesión
    When filtro por session_id
    Then obtengo solo los prompts de esa sesión, ordenados cronológicamente

  Scenario: Paginación de resultados
    When busco prompts con offset y limit
    Then obtengo la página correspondiente
    And el total de resultados disponibles

  Scenario: Eliminar un prompt
    When elimino un prompt por id
    Then el prompt ya no aparece en búsquedas
```

## Análisis breve

- **Qué pide realmente:** Tabla `prompts` similar a observations pero solo para prompts de usuario. Buffer process-local para no bloquear en escritura.
- **Módulos sospechados:** `internal/store/pg/prompt.go`, `internal/memory/prompt.go`, `internal/memory/buffer.go`
- **Riesgos / dependencias:** Parecido a HU-03.1 pero más simple. Buffer process-local requiere manejo de concurrencia y flush on shutdown.
- **Esfuerzo tentativo:** M

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
