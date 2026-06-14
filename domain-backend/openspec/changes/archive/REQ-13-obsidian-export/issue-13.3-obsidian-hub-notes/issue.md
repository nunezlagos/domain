# issue-13.3-obsidian-hub-notes

**Origen:** `REQ-13-obsidian-export`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de Obsidian
**Quiero** tener notas hub de sesión (`_sessions/{id}.md`) que listen todas las observaciones de esa sesión como wikilinks
**Para** navegar rápidamente entre observaciones que pertenecen a una misma sesión de desarrollo

**Como** usuario de Obsidian
**Quiero** tener notas hub de tópico (`_topics/{prefix}.md`) para topic_keys con 2 o más observaciones
**Para** agrupar observaciones relacionadas por tema y descubrir clusters de conocimiento

## Criterios de aceptación

```gherkin
Scenario: Session hub note se crea para cada sesión con observaciones
  Given una sesión "s1" con 3 observaciones
  When se genera el export con hub notes
  Then se crea _sessions/s1.md
  And lista las 3 observaciones como wikilinks [[observations/{slug}]]

Scenario: Session hub note tiene YAML frontmatter correcto
  Given una sesión "s1" con project="Domain"
  When se genera la hub note
  Then el frontmatter contiene:
    | campo      | valor                  |
    |------------|------------------------|
    | type       | session-hub            |
    | session_id | s1                     |
    | tags       | ["session", "Domain"] |
    | created_at | <fecha de la sesión>   |
    | project    | memoria                |

Scenario: Session hub note incluye metadata de la sesión
  Given una sesión con directory="/home/project" y project="Domain"
  When se genera la hub note
  Then el body incluye:
    - Session ID
    - Directorio de trabajo
    - Proyecto
    - Fecha de inicio
    - Fecha de fin (si aplica)
    - Cantidad de observaciones
    - Lista de observaciones como wikilinks

Scenario: Topic hub note se crea para topic_keys con >= 2 observaciones
  Given 3 observaciones con topic_key="auth" y 1 con topic_key="ui"
  When se genera el export con hub notes
  Then se crea _topics/auth.md
  And NO se crea _topics/ui.md (solo 1 obs)

Scenario: Topic hub note lista observaciones como wikilinks
  Given 3 observaciones con topic_key="auth"
  When se genera _topics/auth.md
  Then lista las 3 observaciones como wikilinks

Scenario: Topic hub note tiene YAML frontmatter correcto
  Given topic_key="auth" con project="Domain"
  When se genera _topics/auth.md
  Then el frontmatter contiene:
    | campo        | valor                |
    |--------------|----------------------|
    | type         | topic-hub            |
    | topic_prefix | auth                 |
    | tags         | ["topic", "auth"]    |
    | project      | memoria              |

Scenario: Session sin observaciones no genera hub note
  Given una sesión sin observaciones
  When se genera el export
  Then no se crea _sessions/{id}.md

Scenario: Topic hub note incluye metadatos del cluster
  Given topic_key="auth" con 3 observaciones de 2 sesiones distintas
  When se genera la hub note
  Then incluye:
    - Prefix del topic
    - Cantidad de observaciones
    - Cantidad de sesiones involucradas
    - Lista de observaciones como wikilinks agrupadas por tipo

Scenario: Hub notes se regeneran en cada export
  Given _sessions/s1.md existe de un export anterior
  When se ejecuta export con hub notes
  Then _sessions/s1.md se regenera con observaciones actualizadas

Scenario: Hub notes se ubican en carpetas _sessions/ y _topics/
  Given se generan hub notes
  Then Los archivos están en:
    - {vault}/_sessions/{id}.md
    - {vault}/_topics/{prefix}.md

Scenario: Topic hub note agrupa por type dentro del body
  Given observaciones con topic_key="auth" de types "fix", "feat", "fix"
  When se genera _topics/auth.md
  Then las observaciones se listan agrupadas por type:
    ## fix
    - [[observations/...]]
    - [[observations/...]]
    ## feat
    - [[observations/...]]
```

## Análisis breve

- **Qué pide realmente:** Generar notas índice (hub notes) para sesiones y tópicos. Sessions listan todas las observaciones de esa sesión. Topics listan observaciones agrupadas por topic_key con >= 2 obs. Prefijo `_` para que aparezcan al inicio del explorador de Obsidian.
- **Módulos sospechados:** `internal/obsidian/` — `hub.go` con `GenerateSessionHubs`, `GenerateTopicHubs`
- **Riesgos / dependencias:** Depende de issue-13.1 (slugify, Note rendering, StoreReader) y de topic_key en observations; sesiones sin obs no generan hub; topics con < 2 obs no generan hub
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
- **Evidencia:** —
- **Acción derivada:** —
