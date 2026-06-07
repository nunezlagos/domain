# Proposal: HU-03.2-data-commands

## Intención

Permitir al usuario exportar su memoria completa a JSON (backup/migración), importar datos desde JSON (restore/migración), y gestionar proyectos: listar con conteos, consolidar nombres duplicados (case-insensitive), y limpiar proyectos huérfanos.

## Scope

**Incluye:**

- Comando `export [file]` con flag `--project` — si file se omite, stdout; JSON con sessions, observations, prompts
- Comando `import <file>` — file o `-` para stdin; transaccional con validación previa; INSERT OR IGNORE para sesiones
- Comando `projects list` — lista proyectos con count de observaciones activas
- Comando `projects consolidate` flag `--project` — merge case-insensitive de nombres de proyecto
- Comando `projects prune` flag `--dry-run` — elimina proyectos sin observaciones activas
- Output consistente con HU-03.1 (tablas para listas, líneas para acciones, `--json` global)

**No incluye:**

- HTTP endpoints para export/import (REQ-05)
- Cloud import/export (REQ-09)
- Git sync (REQ-07)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Export store | Reutilizar `store.Export(project)` de HU-01.8 |
| Import store | Reutilizar `store.Import(data)` de HU-01.8 |
| File I/O | `os.WriteFile` y `os.ReadFile`; `-` usa `io.ReadAll(os.Stdin)` |
| JSON format | `{"version": 1, "exported_at": "...", "sessions": [...], "observations": [...], "prompts": [...]}` |
| Projects store | `SELECT project, COUNT(*) FROM observations WHERE deleted_at IS NULL GROUP BY project` |
| Consolidate | `UPDATE observations SET project = LOWER(project) WHERE project != LOWER(project)` |
| Prune | `DELETE FROM ...` projects sin observaciones activas |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Archivos JSON grandes (>100MB) | Baja | Streaming no es necesario inicialmente; lectura completa es aceptable |
| Import parcial por error | Baja | Transacción + validación previa; si falla, rollback total |
| Consolidate afecta proyectos no deseados | Baja | Flag `--dry-run` en consolidate tb; `--project` para acotar |
| Prune elimina metadata de proyecto que el usuario quería mantener | Media | `--dry-run` por defecto? No, pero el output es claro y reversible |

## Testing

- **Export:** test con datos reales en DB, verificar JSON de salida, test con --project, test archivo vs stdout, test DB vacía
- **Import:** test con JSON válido, test stdin, test JSON inválido, test campos faltantes, test duplicados, test transaccional (error a medio camino)
- **Projects:** test list con y sin datos, consolidate con variantes case, prune con dry-run y real
- **Sabotaje:** Import con JSON que viola FK → rollback total → DB no modificada
