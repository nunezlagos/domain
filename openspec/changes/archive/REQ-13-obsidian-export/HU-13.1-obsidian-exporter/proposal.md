# Proposal: HU-13.1-obsidian-exporter

## Intención

Implementar el exportador markdown que convierte observaciones, sesiones y prompts de engram en archivos .md listos para un vault de Obsidian. El corazón del REQ-13: sin exporter, no hay vault. Debe generar wikilinks basados en relaciones, YAML frontmatter con metadatos completos, y soportar exportación incremental e instrumentada con flags.

## Scope

**Incluye:**

- Interfaz `StoreReader` en `internal/obsidian/reader.go` con métodos read-only: `ListObservations`, `GetObservation`, `ListSessions`, `GetSession`, `ListPrompts`, `GetPrompt`
- Paquete `internal/obsidian/` con:
  - `exporter.go` — orquestador `Export(ctx, reader, vaultPath, opts)` que recorre observaciones, calcula slugs, genera archivos
  - `frontmatter.go` — construcción de YAML frontmatter con metadatos
  - `wikilink.go` — generación de wikilinks `[[path/to/note]]` basados en memory_relations
  - `slug.go` — slug determinístico desde título, + desambiguación con ID en colisiones
  - `note.go` — estructura Note con Frontmatter + Body + Wikilinks
- Flags CLI: `--vault` (required), `--project`, `--limit`, `--since`, `--force`, `--include-sessions`, `--include-prompts`
- Exportación incremental: solo exporta observaciones nuevas/modificadas desde última exportación (consulta state file de HU-13.4)
- Manejo de colisiones en slugs con sufijo numérico
- Tests de integración con StoreReader mock

**No incluye:**

- File watcher (HU-13.4)
- Hub notes (HU-13.3)
- Graph JSON (HU-13.2)
- Plugin Obsidian (HU-13.5)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Paquete | `internal/obsidian/` — nuevo package autocontenido |
| StoreReader | Interface en el mismo package; el caller inyecta implementación concreta (store.Store) |
| Slugs | `strings.ToLower + strings.ReplaceAll(" ", "-") + regexp.MustCompile("[^a-z0-9-]").ReplaceAllString` + desambiguación |
| Wikilinks | Formato `[[{type}s/{slug}]]` ej: `[[observations/bug-en-login-modal]]` |
| Frontmatter | `gopkg.in/yaml.v3` con struct ObservationFrontmatter |
| Filename | `{type}s/{slug}.md` ej: `observations/bug-en-login-modal.md` |
| Vault structure | `observations/`, `sessions/`, `prompts/` subdirectorios |
| State | Archivo YAML en vault root `.engram-state.yaml`; lo escribe state-manager de HU-13.4 |
| Colisiones | Mapa `slug -> count` global; si slug ya usado, append `-{count+1}` |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Slugs cambian si cambia el título → wikilinks rotos | Media | Slug se basa en title al momento de exportar; si title cambia, se genera nuevo archivo y el viejo queda huérfano. --force regenera todo |
| Vault enorme (>10k archivos) | Baja | --limit y --since mitigan; export completo es operación batch única |
| Caracteres especiales en frontmatter rompen YAML | Baja | yaml.v3 escapa strings automáticamente; si hay `:` o `#` se manejan con quoting |
| Wikilink a observación que no se exportó en este batch | Media | El wikilink se genera igual — apunta al slug. Si esa obs existe en vault (export anterior), funciona. Si no, es un Dead Link que Obsidian marca pero no rompe nada |

## Testing

- **Unitario:** Slug generation, frontmatter rendering, wikilink generation, desambiguación de colisiones
- **Integración:** Export completo con StoreReader mock que devuelve N observations + relations → verificar archivos generados en temp dir
- **Flags:** Cada flag se prueba con mock reader; verificar que filtros se aplican
- **Incremental:** Export con state file mock → verificar que solo exporta obs newer que last_export
- **Sabotaje:** Romper slug uniqueness → test de colisión cae → restaurar
