# Tasks: issue-21.6-org-schema-decommission

> **Pre:** backup verificado + dry-run en staging. Destructivo e irreversible (fase C).

## Fase A — App sin GUC + drop RLS org
- [ ] **sd-001**: Middleware deja de ejecutar `SET LOCAL app.current_org_id`
- [ ] **sd-002**: Migración `DISABLE ROW LEVEL SECURITY` + `DROP POLICY *_org_isolation` (~20 tablas)
- [ ] **sd-003**: Deploy + verificar app verde sin RLS por org

## Fase B — Quitar threading org_id de Go (por paquete, build verde c/u)
- [ ] **sd-004**: service/* — remover args orgID + `WHERE organization_id` + campos OrganizationID
- [ ] **sd-005**: api/* (handlers, middleware, ctxkeys) — remover org threading
- [ ] **sd-006**: mcp/*, runner/*, scheduler/*, auth/* — idem
- [ ] **sd-007**: `go build ./...` + integración verde; deploys incrementales

## Fase C — Drop columnas y tablas (DESTRUCTIVO)
- [ ] **sd-008**: Migración `DROP COLUMN organization_id` en las 54 tablas (preserva filas)
- [ ] **sd-009**: Migración drop `current_org_id()`, trigger cross-org, satélites (invitations, usage_counters, plans, org_*), y tabla `organizations` (root al final)
- [ ] **sd-010**: Conteo filas pre/post por tabla (debe preservarse)

## Fase D — Periferia
- [ ] **sd-011**: SDKs (go/python/ts): remover model Organization + campo organization_id wire
- [ ] **sd-012**: Seeds/fixtures/tests (~189 refs) — remover org
- [ ] **sd-013**: Docs (docs/db/rls.md, architecture-overview, RFCs) — actualizar

## Verificación final
- [ ] **sd-014**: Schema sin organization_id ni tabla organizations; app + suite verde; deploy VPS
