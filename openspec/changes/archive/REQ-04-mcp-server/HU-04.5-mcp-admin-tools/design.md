# Design: HU-04.5-mcp-admin-tools

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Project resolution | Algoritmo secuencial con steps | Llamar git commands via exec |
| Doctor checks | SQLite PRAGMA + COUNT queries | Os.Stat + queries separadas |
| Conflict storage | Tabla `conflicts` (REQ-10) | Embed en observations |
| Merge atomicity | SQLite transaction | Application-level rollback |
| Error envelope | Struct genérico con optional field | Mapa dinámico |

El algoritmo de resolución es secuencial con early-return: el primer step que produce un resultado definido es el ganador. Git commands se ejecutan via `os/exec` con timeout de 2s.

## Alternativas descartadas

- **Go library go-git:** Dependencia pesada para solo 2 commands (`remote`, `rev-parse`). `os/exec` es más liviano y predecible.
- **Doctor como tool separada con flags:** No, todo vía MCP request/response.
- **Merge sin transaction:** Inconsistencia si el proceso muere a mitad. Transaction obligatoria.

## Diagrama

```
mem_current_project()
  │
  ├─► ENGRAM_PROJECT env set? ──yes──► return { project, source: "env" }
  │ no
  ├─► Config file exists? ──yes──► return { project, source: "config" }
  │ no
  ├─► git remote URL? ──yes──► extract repo name → return { project, source: "git_remote" }
  │ no
  ├─► git root? ──yes──► basename → return { project, source: "git_root" }
  │ no
  ├─► git child repos? ──yes──► return { project, source: "ambiguous" } + warning
  │ no
  └─► dir basename → return { project, source: "dir_basename" }

mem_doctor()
  │
  ├─► PRAGMA page_count * page_size → db_size
  ├─► SELECT COUNT(*) FROM observations, sessions, prompts
  ├─► SELECT COUNT(*) FROM conflicts WHERE status='pending'
  ├─► SELECT schema_version FROM _meta
  ├─► PRAGMA integrity_check → warnings[]
  └─► Return diagnostics object

mem_judge(request)
  │
  ├─► Validate verdict ∈ [keep_newer, keep_older, keep_both, merge, discard]
  ├─► Validate conflict exists and is pending
  ├─► UPDATE conflicts SET status='resolved', verdict, reasoning
  │     └─► Apply action (e.g., delete if discard, keep fields if keep_newer)
  └─► Return { conflict_id, status: "resolved", verdict }

mem_compare(request)
  │
  ├─► Validate both observation IDs exist
  ├─► Validate verdict ∈ [related, duplicate, unrelated, parent_of, child_of]
  ├─► INSERT INTO relationships (source_id, target_id, verdict, reasoning)
  └─► Return { relationship_id, verdict }

mem_merge_projects(request)
  │
  ├─► Parse "from" as comma-separated project list
  ├─► BEGIN TRANSACTION
  ├─► UPDATE observations SET project=to WHERE project IN (from)
  ├─► UPDATE sessions SET project=to WHERE project IN (from) (if applicable)
  ├─► COMMIT
  └─► Return { merged_count, from_projects, to_project, details: { from: count } }
```

## TDD plan

**Red:**
1. `TestCurrentProjectEnv`: ENGRAM_PROJECT set → source="env"
2. `TestCurrentProjectGitRoot`: inside git repo → source="git_root"
3. `TestCurrentProjectDirBasename`: no git → source="dir_basename"
4. `TestDoctorBasic`: todos los campos esperados
5. `TestDoctorCorruptDB`: integrity_check → warnings[] no vacío
6. `TestJudgeValid`: resolver conflicto → status=resolved
7. `TestJudgeInvalidVerdict`: error
8. `TestJudgeNotFound`: error
9. `TestCompareValid`: crear relación
10. `TestCompareNotFound`: error
11. `TestMergeProjects`: mover 3 → 1, verificar conteos
12. `TestMergeAtomicRollback`: error en mitad → rollback

**Green:** Implementar resolvers, doctor queries, judge/compare store.

**Refactor:** Extraer project resolver a paquete separado `internal/project`.

**Sabotaje:** merge_projects con from vacío → 0 merged. Romper doctor → warning inesperado.

## Riesgos y mitigación

- **Git command timeout:** `context.WithTimeout(ctx, 2*time.Second)` para cada exec. Si timeout, pasa al siguiente step.
- **Config file path:** Buscar `.memoria/config` desde cwd hacia arriba (como git busca `.git`).
- **merge_projects con FROM igual a TO:** No-op, merged_count=0, warning.
