# Proposal: HU-04.5-mcp-admin-tools

## Intención

Exponer 5 herramientas MCP administrativas: detección de proyecto actual (`mem_current_project`), diagnóstico del sistema (`mem_doctor`), resolución de conflictos (`mem_judge`), comparación semántica (`mem_compare`), y consolidación de proyectos (`mem_merge_projects`).

## Scope

**Incluye:**
- `mem_current_project`: Resolver el proyecto activo desde el working directory usando la cadena: config file > git_remote > git_root > git_child > ambiguous heuristic > dir_basename.
- `mem_doctor`: Diagnóstico read-only: path DB, tamaño, conteos, engine, FTS status, schema version, conflictos pendientes.
- `mem_judge`: Resolver conflicto pendiente: almacenar verdict (keep_newer, keep_older, keep_both, merge, discard) + reasoning.
- `mem_compare`: Registrar relación semántica entre dos observaciones con verdict + reasoning.
- `mem_merge_projects`: Migrar observaciones de uno o más proyectos fuente a un proyecto destino canónico. Atómico (transaction).
- Error envelopes: cuando una tool devuelve error, incluir `available_projects` si es relevante.

**Excluye:**
- CRUD (HU-04.2)
- Búsqueda (HU-04.3)
- Sesiones (HU-04.4)
- Profiles/resolution (HU-04.6)

## Enfoque técnico

1. **Project resolution:** Algoritmo secuencial en `internal/project/resolver.go`:
   - `ENGRAM_PROJECT` env var → override inmediato
   - Config file `.memoria/config` → project override
   - `git remote get-url origin` → extraer repo name
   - `git rev-parse --show-toplevel` → basename del root
   - Si hay child git repos → marcar ambiguous
   - Fallback: `os.Getwd()` basename
2. **Doctor:** Queries SQLite de metadatos (PRAGMA page_count, page_size), conteos, check de integridad.
3. **Judge:** Validar verdict contra `[keep_newer, keep_older, keep_both, merge, discard]`. UPDATE conflict SET status=resolved.
4. **Compare:** INSERT en tabla `relationships` (source_id, target_id, verdict, reasoning, created_at).
5. **Merge:** BEGIN TRANSACTION → UPDATE observations SET project=to WHERE project IN (from) → COMMIT.
6. **Error envelope:** Struct `ErrorResponse { error: string, available_projects?: []string }`.

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Merge_projects sin transacción | Data inconsistency | SQLite transaction con rollback on error |
| Doctor accede a DB corrupta | Crash | PRAGMA integrity_check catcher |
| Judge sin REQ-10 completo | No hay conflictos | Judge funcional pero sin datos — no bloquea |
| Current_project en dir sin git | Siempre dir_basename | Comportamiento correcto, documentado |

## Testing

- **Unit:** Resolver mockeado (simular git, config, env). Doctor con store in-memory. Judge/Compare con fixtures. Merge con datos de prueba.
- **Integration:** Server real, secuencia doctor → current_project → judge → compare → merge.
- **Sabotaje:** merge_projects con from vacío → merged_count=0. current_project sin git y sin dir → error.
