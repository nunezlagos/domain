# Design: issue-21.6-org-schema-decommission

## Decisión arquitectónica

**Migración por fases idempotentes, cada una deployable, con la app siempre en estado
consistente.** Nunca se dropea una columna mientras el código todavía la lee. El orden
es: (1) app deja de exigir RLS/GUC, (2) app deja de leer/escribir `organization_id`,
(3) recién entonces se dropea la columna y la tabla.

Esto evita el anti-patrón de "drop schema + arreglar 658 refs de una" que dejaría el
build roto y el deploy caído.

## Orden de operaciones (crítico por FKs y RLS)

```
1. App: quitar SET LOCAL app.current_org_id          (deploy)
2. DB:  DISABLE RLS + DROP POLICY *_org_isolation     (migración N)
3. App: quitar WHERE organization_id por paquete      (deploys incrementales)
4. DB:  DROP COLUMN organization_id (54 tablas)        (migración N+1)  ← preserva filas
5. DB:  DROP FUNCTION current_org_id()                 (migración N+2)
        DROP TRIGGER projects_client_same_org_check
        DROP TABLE invitations, usage_counters, org_*  (satélites primero)
        DROP TABLE organizations                       (root al final)
6. App/SDK/seeds/tests/docs: limpieza final           (deploy)
```

## Por qué fases y no un solo big-bang

- 54 tablas × DROP COLUMN + 658 refs Go en un commit = build roto garantizado y rollback
  imposible si algo falla a mitad.
- Cada fase deja el sistema funcionando: tras fase A la app corre sin RLS; tras fase B
  ya no toca la columna; recién en fase C se dropea.

## Manejo de columnas nullable (SET NULL)

Tablas con `organization_id` nullable (audit_log, event_log, auth_events, notification_deliveries,
hu_drafts, imported_workflow_files): el DROP COLUMN es directo (no hay FK que respetar tras
dropear la tabla organizations). Sin decisión especial: se dropea la columna.

## Rollback

- Fases A–B: revertibles por deploy del binario anterior + re-ENABLE RLS (migración down).
- Fase C (DROP COLUMN/TABLE): **NO revertible sin restore de backup**. De ahí el backup
  obligatorio y el dry-run en staging.

## Alternativas descartadas

- **Mantener `organization_id` con DEFAULT constante (no dropear):** deja deuda y columnas
  muertas en 54 tablas; el objetivo de la HU es justamente eliminarlas.
- **Big-bang en una migración + un commit Go:** inviable con 658 refs; build roto.

## Verificación por fase

| Fase | Check |
|------|-------|
| A | app responde sin GUC; RLS deshabilitada (`\d+ tabla` sin policies) |
| B | `go build ./...` + integración verde por paquete |
| C | conteo filas pre/post igual; `\d tabla` sin columna; tablas org inexistentes |
| D | SDK builds; tests verdes; docs actualizadas |
