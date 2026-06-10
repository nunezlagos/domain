# issue-13.1-obsidian-exporter

**Origen:** `REQ-13-obsidian-export`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de engram
**Quiero** exportar mis observaciones, sesiones y prompts como archivos markdown con wikilinks y YAML frontmatter en un vault de Obsidian
**Para** poder explorar, relacionar y visualizar mi memoria de desarrollo desde Obsidian

**Como** usuario power
**Quiero** controlar la exportación con flags `--vault`, `--project`, `--limit`, `--since`, `--force`
**Para** exportar solo lo relevante, evitar regenerar lo que ya existe y sobrescribir cuando sea necesario

## Criterios de aceptación

```gherkin
Scenario: Export observations to markdown files
  Given hay 3 observaciones en la store
  When ejecuto `engram obsidian export --vault /path/to/vault`
  Then se crean 3 archivos .md en la carpeta observations/ del vault
  And cada archivo tiene YAML frontmatter con type, project, created_at, topic_key
  And cada archivo tiene wikilinks a otras observaciones relacionadas

Scenario: YAML frontmatter contiene metadatos completos
  Given una observación con type="fix", project="Domain", topic_key="auth"
  When se exporta a markdown
  Then el frontmatter contiene:
    | campo       | valor    |
    |-------------|----------|
    | type        | fix      |
    | project     | memoria  |
    | created_at  | <timestamp> |
    | topic_key   | auth     |
    | id          | <id>     |
    | title       | <title>  |
    | scope       | project  |
    | session_id  | <sid>    |

Scenario: Wikilinks entre observaciones relacionadas
  Given obs A (id=1) y obs B (id=5) tienen una relación en memory_relations
  When se exportan ambas a markdown
  Then A.md contiene un wikilink `[[observations/5-title]]`
  And B.md contiene un wikilink `[[observations/1-title]]`

Scenario: Wikilink se genera con slug del título
  Given una observación con title="Bug en login modal"
  When se exporta
  Then el slug generado es "bug-en-login-modal"
  And el wikilink usa `[[observations/bug-en-login-modal]]`

Scenario: Session se exporta como markdown con sus observaciones
  Given una sesión con 2 observaciones
  When ejecuto export con --include-sessions
  Then se crea sessions/{session_id}.md
  And el archivo contiene la metadata de la sesión
  And lista las 2 observaciones como wikilinks

Scenario: Prompt se exporta como markdown
  Given un prompt capturado en user_prompts
  When ejecuto export con --include-prompts
  Then se crea prompts/{prompt_id}.md
  And contiene el contenido del prompt en frontmatter

Scenario: --project filtra por proyecto
  Given observaciones de project "Domain" y project "other"
  When ejecuto `export --project memoria`
  Then solo se exportan observaciones de project "Domain"

Scenario: --limit limita cantidad de observaciones exportadas
  Given 100 observaciones
  When ejecuto `export --limit 10`
  Then solo se exportan 10 archivos markdown

Scenario: --since filtra por fecha
  Given observaciones de ayer y de hace un mes
  When ejecuto `export --since 24h`
  Then solo se exportan observaciones de las últimas 24 horas

Scenario: --force sobrescribe archivos existentes
  Given observations/1-title.md ya existe
  When ejecuto `export --force`
  Then el archivo se sobrescribe con el contenido actualizado

Scenario: Sin --force, archivos existentes no se sobrescriben
  Given observations/1-title.md ya existe con contenido viejo
  When ejecuto `export` sin --force
  Then el archivo NO se modifica

Scenario: --vault PATH es requerido
  When ejecuto `export` sin --vault
  Then recibo un error "flag --vault is required"

Scenario: StoreReader es interfaz read-only
  When reviso la definición de StoreReader
  Then solo expone métodos de lectura: ListObservations, GetObservation, ListSessions, GetSession, ListPrompts, GetPrompt

Scenario: Exportación incremental respeta state file
  Given state file indica última exportación hace 1 hora
  When ejecuto `export` sin --force ni --since
  Then solo se exportan observaciones creadas/modificadas después de la última exportación

Scenario: Slugs colisionantes se desambiguan con ID
  Given dos observaciones con título "Bug fix"
  When se exportan
  Then los archivos son "bug-fix.md" y "bug-fix-2.md"

Scenario: Wikilink apunta a archivo correcto aunque cambie el slug
  Given una observación relacionada
  When se genera el wikilink
  Then usa el mismo slug que el filename real
```

## Análisis breve

- **Qué pide realmente:** Un exportador bidireccional: engram → Obsidian vault como archivos markdown. Debe convertir observaciones, sesiones y prompts. Generar wikilinks basados en relaciones existentes (memory_relations). YAML frontmatter completo para que Obsidian lo indexe correctamente. StoreReader interface para garantizar que el exportador solo lee, nunca escribe en la store.
- **Módulos sospechados:** `internal/obsidian/` — `exporter.go`, `types.go`, `frontmatter.go`, `wikilink.go`, `slug.go`; interfaz `StoreReader` en `internal/obsidian/reader.go`
- **Riesgos / dependencias:** Depende de observations CRUD (issue-01.2) y memory_relations (issue-10.x) para datos fuente; la generación de slugs debe ser consistente y determinística; archivos colisionantes deben desambiguarse; state file compartido con issue-13.4
- **Esfuerzo tentativo:** L

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
