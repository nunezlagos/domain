# Tasks: issue-42.1-taxonomia-y-catalogo

## Verificación previa (bloqueante)

- [ ] Confirmar que la próxima migration libre es `000147` (`git ls-files .../migrations | sort | tail` → última 000146)
- [ ] Confirmar conteo real de tablas (esperado 97) contra la introspección del schema real
- [ ] Confirmar que NO existe ya `table_catalog` (`SELECT to_regclass('table_catalog')` → NULL)
- [ ] Verificar que CADA nombre del seed coincide EXACTAMENTE con el nombre actual en el schema (no el propuesto)
- [ ] Confirmar que las 14 tablas marcadas para DROP NO están en el seed
- [ ] Confirmar nombres "trampa" actuales: `org_enrollment_tokens` (no `enrollment_tokens`), `verifications` (no `tdd_verifications`), `requirements` (no `sdd_requirements`)

## Migration 000147 (up)

- [ ] Crear `000147_create_table_catalog.up.sql` con header completo (migration/author/issue/description/breaking/estimated_duration)
- [ ] `BEGIN; ... COMMIT;` (una sola transacción)
- [ ] `CREATE TABLE IF NOT EXISTS table_catalog (table_name text PRIMARY KEY, grupo text NOT NULL, label text NOT NULL, sort_order integer NOT NULL)`
- [ ] `COMMENT ON TABLE/COLUMN` documentando que es source of truth y que table_name se actualiza con cada rename
- [ ] `INSERT INTO table_catalog ... ON CONFLICT (table_name) DO UPDATE` con las ~70 tablas conservadas (nombres ACTUALES)
- [ ] Excluir del INSERT las tablas de DROP (billing + legacy/infra)
- [ ] `sort_order` por bloques de 100 por grupo (users=100, auth=200, ..., internal=9900)

## Migration 000147 (down)

- [ ] Crear `000147_create_table_catalog.down.sql` con header
- [ ] `BEGIN; DROP TABLE IF EXISTS table_catalog; COMMIT;`
- [ ] Verificar que el down NO toca ninguna otra tabla

## Validación de la migration

- [ ] `squawk` sobre el par up/down sin findings bloqueantes
- [ ] Aplicar `up` en entorno local/efímero → `to_regclass('table_catalog')` no es NULL
- [ ] `SELECT count(*) FROM table_catalog` == nº de tablas conservadas sembradas
- [ ] `SELECT count(DISTINCT grupo) FROM table_catalog` == 23 (incluye internal)
- [ ] Aplicar `down` → `to_regclass('table_catalog')` es NULL, resto intacto
- [ ] Reaplicar `up` dos veces seguidas → mismo conteo (idempotencia ON CONFLICT)

## Consistencia con la taxonomía (doc)

- [ ] Verificar que `issue.md` (Mapa de taxonomía) y el seed de la migration listan exactamente las mismas tablas/grupos
- [ ] Verificar que cada tabla del seed tiene label == label del grupo en el mapa
- [ ] Verificar que ninguna tabla aparece en dos grupos

## Sabotaje (anti-falsos positivos)

Objetivo: probar que el seed refleja el schema REAL (pre-rename) y NO el propuesto, y que el catálogo no inventa ni omite tablas.

- [ ] **Sabotaje 1 (nombre propuesto vs actual):** editar temporalmente el seed para insertar `'enrollment_tokens'` (nombre PROPUESTO) en vez de `'org_enrollment_tokens'` (actual). El test de consistencia debe FALLAR con "tabla del catálogo no existe en el schema" / "esperado org_enrollment_tokens". Restaurar `org_enrollment_tokens` → test pasa.
- [ ] **Sabotaje 2 (tabla a dropear filtrada):** agregar temporalmente `('plans','billing','Billing',9999)` al seed. El test "tablas de DROP fuera del catálogo" debe FALLAR (`count > 0`). Quitar la fila → test pasa.
- [ ] **Sabotaje 3 (delta de conteo):** insertar un `ALTER TABLE users RENAME TO users_users` falso dentro de 000147. El test "el catálogo NO renombra (delta de tablas == +1, ningún rename)" debe FALLAR. Quitar el ALTER → test pasa.
- [ ] Confirmar que los 3 sabotajes, una vez restaurados, dejan la suite VERDE (no quedó un assert comentado ni un skip).

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK
- [ ] `go test ./internal/migrate/...` verde (test de up/down/idempotencia/seed)
- [ ] `squawk` verde sobre 000147 up+down
- [ ] NO renames, NO drops aplicados (esta HU es aditiva pura)
- [ ] Commit en rama `services`: `feat(req-42.1): tabla table_catalog + seed de taxonomía (source of truth)`
- [ ] NO git push (repo local-only)
