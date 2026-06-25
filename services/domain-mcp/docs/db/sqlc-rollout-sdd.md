# SDD — Rollout sqlc a la capa de datos

Spec-Driven Development para migrar los 44 services restantes de `internal/service/*`
al patrón sqlc. El patrón técnico está en [sqlc-data-layer.md](./sqlc-data-layer.md);
este doc es el **plan de ejecución**: inventario, olas, criterios de aceptación y tracking.

## Objetivo

Toda query SQL del backend queda en un catálogo `.sql` nombrado y type-safe. Se
elimina el SQL inline + `rows.Scan` posicional. Resultado: queries identificadas
(auditables, optimizables) y cambios de schema sin bugs de mapeo.

## No-objetivos

- No cambiar la API pública de ningún service (los callers — handlers, MCP tools —
  no se tocan). Cada migración es interna al service.
- No cambiar el comportamiento RLS ni transaccional observable.
- No tocar `domain-admin` (Python) — es otro límite, ver riesgo Go↔Python.

## Patrón canónico de migración (resumen)

Por cada service:

1. `sqlc.yaml` (copiar de `issue`, ajustar `package`/`out`) apuntando a
   `../../migrate/migrations`.
2. `sql/query.sql` — mover cada query inline a una entrada `-- name: X :one|:many|:exec|:execrows`.
   Ordenar columnas igual al orden físico de la tabla para reutilizar el struct de tabla.
   Filtros opcionales con `sqlc.narg`.
3. `make sqlc` (o `go generate ./...` en el dir) → genera `<name>db/`.
4. Reescribir `service.go`: reemplazar SQL inline por llamadas a `q(...)`,
   mapear el struct generado al de dominio (`toX`).
5. **Acceso a la conexión:**
   - **Tabla SIN RLS** → `q := xxxdb.New(s.Pool)` (como `issue`).
   - **Tabla CON RLS** → helper que honra el tx-context:
     ```go
     func (s *Service) q(ctx context.Context) *xxxdb.Queries {
         if tx := txctx.TxFromContext(ctx); tx != nil {
             return xxxdb.New(tx)   // tx con SET LOCAL app.current_org_id → RLS aplica
         }
         return xxxdb.New(s.Pool)   // fallback (paths sin org-tx)
     }
     ```
   - Transacción multi-tabla propia → `xxxdb.New(tx)` con la tx abierta en el service.

### Criterios de aceptación (por service)

- [ ] `service.go` sin strings SQL ni `rows.Scan` posicional.
- [ ] `go build ./...` verde (no se rompió ningún caller → API pública intacta).
- [ ] `go test ./internal/service/<name>/...` verde.
- [ ] `make sqlc` idempotente (regenerar no produce diff).
- [ ] Services con RLS: las Queries reciben la tx del context, no el pool crudo.
- [ ] Commit aislado por service (o por ola chica): `refactor(<name>): capa de datos a sqlc`.

## Tablas con RLS FORCE (20)

`activity_log, api_keys, audit_log, captured_prompts, clients, observations,
organizations, otp_codes, project_index_runs, project_policies,
project_policy_versions, project_repositories, projects, project_ticket_comments,
project_tickets, project_ticket_status_history, secrets, sessions, users, verifications`

Un service que toque cualquiera de estas → usar el helper `q(ctx)` con txctx.

## Inventario y olas

Leyenda: **Q** = nº de queries SQL · **LOC** = service sin tests · **RLS** = toca tabla RLS.

### ✅ Ola 0 — Piloto (hecho)
| Service | Q | LOC | RLS | Commit |
|---|--:|--:|:--:|---|
| issue | 12 | 917 | no | `02715370` |

### Ola 1 — Quick wins (inline, sin RLS, ≤10 Q)
| Service | Q | LOC | RLS | Estado |
|---|--:|--:|:--:|:--:|
| mcpinstaller | 1 | 261 | no | ⬜ |
| projecttemplate | 5 | 162 | no | ⬜ |
| policy | 8 | 232 | no | ⬜ |
| mcpserver | 10 | 359 | no | ⬜ |
| requirement | 10 | 402 | no | ⬜ |

### Ola 2 — Medianos inline, sin RLS
| Service | Q | LOC | RLS | Estado |
|---|--:|--:|:--:|:--:|
| usagealerts | 13 | 651 | no | ⬜ |
| spec | 17 | 379 | no | ⬜ |
| task | 19 | 414 | no | ⬜ |
| traceability | 28 | 423 | no | ⬜ |

### Ola 3 — Inline CON RLS, chicos/medianos (usar `q(ctx)`)
| Service | Q | LOC | Estado |
|---|--:|--:|:--:|
| promptrouter | 3 | 673 | ⬜ |
| cost | 4 | 98 | ⬜ |
| search | 5 | 296 | ⬜ |
| billing | 7 | 174 | ⬜ |
| inventory | 8 | 266 | ⬜ |
| projectlink | 8 | 118 | ⬜ |
| timeline | 9 | 307 | ⬜ |
| enrollment | 9 | 420 | ⬜ |
| projectmerge | 10 | 248 | ⬜ |
| extsync | 11 | 317 | ⬜ |
| intake | 11 | 347 | ⬜ |
| attachment | 11 | 280 | ⬜ |
| usage | 12 | 333 | ⬜ |
| webhook | 13 | 344 | ⬜ |
| knowledge | 14 | 371 | ⬜ |
| prompt | 14 | 383 | ⬜ |
| workflowimport | 14 | 397 | ⬜ |

### Ola 4 — Inline CON RLS, grandes
| Service | Q | LOC | Estado |
|---|--:|--:|:--:|
| lifecycle | 21 | 462 | ⬜ |
| issuebuilder | 21 | 1584 | ⬜ |
| outboundwebhook | 29 | 757 | ⬜ |
| cron | 31 | 574 | ⬜ |
| skill | 32 | 1305 | ⬜ |

### Ola 5 — Ya tienen repository (refactor pg_repository → sqlc)
Más mecánicos: el SQL ya está centralizado en `pg_repository.go`; se mueve a `query.sql`.
Todos tocan RLS (mantener el switch `q(ctx)` que ya tienen).
| Service | Q | LOC | Estado |
|---|--:|--:|:--:|
| projectpolicy | 7 | 349 | ⬜ |
| project | 9 | 560 | ⬜ |
| projectrepo | 9 | 445 | ⬜ |
| capturedprompt | 6 | 373 | ⬜ |
| client | 10 | 726 | ⬜ |
| observation | 14 | 603 | ⬜ |
| ticket | 22 | 1125 | ⬜ |
| agent | 19 | 1379 | ⬜ |

### Ola 6 — Monstruos (sesión dedicada c/u)
| Service | Q | LOC | RLS | Estado |
|---|--:|--:|:--:|:--:|
| orchestrator | 44 | 4260 | sí | ⬜ |
| flow | 61 | 3002 | no | ⬜ |

### N/A — Verificar (0 queries SQL directas)
`imports`, `projectdetect`, `wizardplan` — probablemente delegan en otros services.
Confirmar que no tienen acceso directo a DB; si así es, no migran.

## Verificación end-to-end (por ola)

```bash
cd services/domain-mcp
make sqlc                              # regenera; debe ser idempotente
go build ./...                         # ningún caller roto
go test -short ./internal/service/...  # tests de services
git status --short                     # solo los archivos esperados
```

Para RLS, el test de integración (`make test-integration`, requiere Docker) valida
que las queries devuelven filas dentro de `txctx.WithOrgTx` y 0 filas fuera.
