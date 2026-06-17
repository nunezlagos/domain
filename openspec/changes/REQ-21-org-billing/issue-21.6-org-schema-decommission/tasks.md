# Tasks: issue-21.6-org-schema-decommission

> **Pre:** backup verificado + dry-run en staging. Destructivo e irreversible (fase C).

## Fase A — App sin GUC + drop RLS org
- [~] **sd-001**: DIFERIDO a Fase B. Quitar `set_config('app.current_org_id', ...)` del middleware
  (hot path de auth) no aporta beneficio funcional con RLS ya deshabilitada y agrega riesgo;
  además el mismo `openTxWithOrg` setea `app.current_user_id` que SIGUE necesitando la RLS de
  otp_codes (user-isolation). Se remueve junto con el threading de queries en Fase B.
- [x] **sd-002**: Migración `000132_disable_org_rls` — `DISABLE ROW LEVEL SECURITY` en las 19 tablas
  org-scoped (NO otp_codes, que es user-isolation). Reversible: no dropea policies (quedan inertes);
  el down hace ENABLE+FORCE. El DROP POLICY definitivo + `current_org_id()` va en Fase C.
- [x] **sd-003**: `go build ./...` Success + `go vet` default sin issues. Deploy diferido (decisión operador).

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
