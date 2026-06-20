# RFC 0007 — Rename `HU` → `issue`

**Status:** draft
**Author:** nunezlagos
**Created:** 2026-06-10
**Decided:** 2026-06-10 (user opt B en discusión RFC 0006)
**Blocks:** RFC 0006 (specs derivadas esperan este rename)

## Motivación

Hoy Domain usa `HU` (issue) como unidad básica de SDD. La regla `.claude/rules/sdd.md` admite 6 tipos:

```
feature | infrastructure | hardening | tooling | docs | runbook
```

De estos, sólo `feature` encaja semánticamente con "issue" BDD pura. Los otros 5 son tareas técnicas que NO tienen "Como X quiero Y para Z" natural. El naming `HU` está estirado.

**`issue` es más universal:**
- Match natural con Jira / GitHub / Linear (que Domain ya sinca vía REQ-04 `external_sync`)
- Engloba todos los tipos sin forzar metáforas
- Cero costo cognitivo para usuarios que vienen de esas plataformas
- Alineación con cómo el resto de la industria habla

## Scope

Renombrado **integral** del concepto en:

| Área | Hoy | Después |
|---|---|---|
| Tabla BD | `issues` | `issues` |
| Tabla BD | `gherkin_scenarios.issue_id` | `gherkin_scenarios.issue_id` |
| Tabla BD | `proposals.issue_id` | `proposals.issue_id` |
| Tabla BD | `designs.issue_id` | `designs.issue_id` |
| Tabla BD | `tasks.issue_id` | `tasks.issue_id` |
| Tabla BD | `code_references.issue_id` | `code_references.issue_id` |
| Tabla BD | `issue_drafts` | `issue_drafts` |
| Tabla BD | `issue_drafts.id` referenciado | `issue_drafts.id` |
| Tabla BD | `entity_state_transitions.entity_kind='hu'` | `entity_kind='issue'` |
| Service | `internal/service/issuebuilder/` | `internal/service/issuebuilder/` |
| Service | `internal/service/issuebuilder.Service.PromoteAttachmentsToHU` | `PromoteAttachmentsToIssue` |
| Paths spec | `openspec/changes/REQ-XX/issue-XX.Y-slug/` | `openspec/changes/REQ-XX/issue-XX.Y-slug/` |
| Slugs internos | `issue-XX.Y` (con punto) | `issue-XX.Y` (mantiene formato numerado) |
| MCP tools | `domain_hu_*` (si existieran) | `domain_issue_*` |
| Headers HU.md | `# issue-XX.Y-slug` | `# issue-XX.Y-slug` |
| Rule .md | `.claude/rules/sdd.md` (texto "HU") | "issue" |
| Diagramas | "HU implementada" | "issue implementada" |

## Lo que NO cambia

- Formato numerado `XX.Y` (REQ-08, issue-08.10) — mantiene
- REQ (requerimientos) — sigue siendo `REQ-XX-slug`
- Slug schema (snake-case en BD, kebab-case en paths)
- Formato Gherkin de escenarios — sigue válido
- `tasks` (subtareas atómicas) — el nombre tabla ya está bien
- Naming externo: cuando se sinca con Jira, sigue siendo "Jira issue"; cuando con GitHub, "GitHub issue". El mapping 1:1 es ahora más limpio.

## Migration plan

### Fase 1: Schema BD (1 migration)

```sql
-- migration 000074_rename_hu_to_issue.up.sql
BEGIN;

ALTER TABLE issues RENAME TO issues;
ALTER TABLE issue_drafts RENAME TO issue_drafts;
ALTER TABLE hu_draft_steps_log RENAME TO issue_draft_steps_log;
ALTER TABLE issue_draft_steps_log RENAME COLUMN draft_id TO issue_draft_id;

ALTER TABLE gherkin_scenarios RENAME COLUMN issue_id TO issue_id;
ALTER TABLE proposals RENAME COLUMN issue_id TO issue_id;
ALTER TABLE designs RENAME COLUMN issue_id TO issue_id;
ALTER TABLE tasks RENAME COLUMN issue_id TO issue_id;
ALTER TABLE code_references RENAME COLUMN issue_id TO issue_id;

-- entity_state_transitions data migration
UPDATE entity_state_transitions SET entity_kind = 'issue' WHERE entity_kind = 'hu';

-- Indexes renombrados (Postgres lo hace automático con rename de columna/tabla, pero verificar)
-- Triggers updated_at - se renombran automáticamente con la tabla

COMMIT;
```

`down.sql`: inverso exacto.

### Fase 2: Code Go

Renombres en orden:
1. `internal/service/issuebuilder/` → `internal/service/issuebuilder/`
2. Tipos `IssueBuilder*` → `IssueBuilder*`
3. Funciones `*HU*` → `*Issue*`
4. Tags JSON `"issue_id"` → `"issue_id"` en structs
5. SQL queries con `issue_id` → `issue_id`
6. Variables locales `huID`, `hu` → `issueID`, `issue`
7. Comments y strings

Herramienta: `gopls rename` + `gofmt` + tests.

### Fase 3: Specs y docs

```bash
# rename paths
find openspec/changes -depth -name "issue-*" -execdir bash -c \
  'mv "$0" "${0/issue-/issue-}"' {} \;

# rename contenido archivos
find openspec/changes -name "*.md" -exec sed -i 's/issue-/issue-/g' {} \;
find openspec/changes -name "*.md" -exec sed -i 's/issue/issue/gi' {} \;
find docs -name "*.md" -exec sed -i 's/issue-/issue-/g' {} \;
find .claude/rules -name "*.md" -exec sed -i 's/issue-/issue-/g' {} \;
```

### Fase 4: MCP tools y API

- Tools MCP `domain_hu_*` (verificar si existen) → renombrar a `domain_issue_*`
- API HTTP: si hay rutas `/api/v1/hus/...` → `/api/v1/issues/...`
- Mantener aliases temporales con `Deprecation` header por 1 release

### Fase 5: CLI

```bash
./bin/domain hu list   # alias deprecated, warn
./bin/domain issue list  # nuevo canonical
```

## Backward compatibility

Como Domain es **local-only sin prod ni usuarios externos**, no necesita backwards-compat real. El rename puede ser hard:

- Sin alias deprecated en MCP tools (no hay clientes externos)
- Sin support por X releases (no hay X releases)
- Migration 000074 corre una vez, todos los environments están en dev

Si en el futuro Domain abre a usuarios externos, se documenta el cambio en CHANGELOG y los users hacen `migrate up` una vez.

## Riesgos

| Riesgo | Mitigación |
|---|---|
| Sed-rename en paths puede romper imports Go | `gopls rename` primero (sintáctico), después `sed` en docs no-code |
| Tests rotos | Correr suite completa post-rename. Tests E2E (39s) detectan |
| Commits in-flight con paths viejos | Sacar a una sola PR atómica, después merge |
| `git log --follow` se rompe en archivos renombrados | Aceptable; el rename es trazable en el commit |
| Engram memory tiene referencias a "HU" | Persiste — engram es historia, no source of truth. Las memories viejas siguen siendo válidas con naming antiguo. |

## Esfuerzo estimado

| Fase | Tiempo |
|---|---|
| Migration 000074 + tests | 30 min |
| Code Go (rename + tests verdes) | 60 min |
| Specs y docs (find/sed) | 30 min |
| MCP + API + CLI | 30 min |
| Smoke test E2E | 20 min |
| **Total** | **~3 horas** |

## Decisión

**Status:** approved by user (RFC 0006 discussion 2026-06-10).

## Próximos pasos

1. Crear `issue-X.Y-rename-hu-to-issue` (issue propia bajo nuevo naming, post-migration)
2. Ejecutar Fase 1 (migration 000074)
3. Ejecutar Fase 2 (code Go con gopls rename)
4. Ejecutar Fase 3 (specs y docs)
5. Ejecutar Fase 4-5 (MCP/API/CLI)
6. Test E2E suite completa verde
7. Commit atómico `refactor(rename): HU → issue` + tag pre-release
8. **Desbloquear RFC 0006:** crear `issue-08.10`, `issue-08.11`, `issue-08.12`

## Referencias

- RFC 0006 (bloqueado por este) — `docs/rfc/0006-sdd-pipeline-orchestrator.md`
- `.claude/rules/sdd.md` — define los 6 tipos
- REQ-04 external_sync — mapping HU ↔ Jira/GitHub/Linear issue
- Inventario BD — `issues`, `issue_drafts`, `hu_draft_steps_log` + columnas `issue_id` en 5 tablas
