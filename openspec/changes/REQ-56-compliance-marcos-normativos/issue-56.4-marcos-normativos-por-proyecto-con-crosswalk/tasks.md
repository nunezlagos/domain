# Tasks: marcos normativos por proyecto con crosswalk

## Tests (van primero — red)

- [ ] Test de integración: el catálogo se lee SIN `app.current_project_id` seteado (REQ-4)
- [ ] Test de integración: `project_compliance_frameworks` de otro proyecto NO es visible (REQ-4)
- [ ] Test de integración: insertar en `project_control_status` sin GUC es rechazado (REQ-4, sabotaje)
- [ ] Test: proyecto sin filas declaradas devuelve cero marcos aplicables (REQ-1)
- [ ] Test: un control `ok` se reporta cumplido en los N marcos que lo exigen, con su referencia propia (REQ-2)
- [ ] Test: ingesta rechazada para un marco `solo_referencia`, aceptada para `texto_libre` (REQ-3)
- [ ] Test: marco con `vigente_desde` futuro se reporta aparte y no penaliza el score (REQ-5)
- [ ] Test: dos ediciones de la misma norma conviven y no mezclan cláusulas (REQ-6)

Los de RLS con `//go:build integration` y testcontainers: la suite unitaria pasa entera con el RLS
mal puesto (medido en DOMAINSERV-240).

## Implementación

- [ ] Migración `000291`: `compliance_frameworks`, `compliance_controls`, `framework_controls`
      (catálogo, sin RLS) — header obligatorio, índices con `CONCURRENTLY` salvo justificación
- [ ] Migración `000291` (cont.): `project_compliance_frameworks`, `project_control_status` con
      `ENABLE`/`FORCE ROW LEVEL SECURITY` + policy por `current_project_id()`
- [ ] `internal/service/compliance`: resolución de marcos aplicables + expansión del crosswalk
- [ ] Guard de `fuente_tipo` en el camino de ingesta a knowledge
- [ ] Tools MCP `domain_compliance_framework_list` / `_project_set` / `_control_status_set` /
      `_report`, todas bajo `rlsProyecto`
- [ ] Registrar las tools en `toolGroups`, en `tool_channels.go` Y en `TOOL_CHANNELS.md`
      (el guard exige las tres, con el count del header actualizado)
- [ ] Seeder del catálogo inicial: `ley-21719`, `ley-21595`, `gdpr` + controles compartidos
      (seeder + bump, no SQL suelto — `data-migration-methodology`)

## Verify (auditoría, última task — `audit-tasks-checklist`)

- [ ] Ninguna función nueva supera 50 líneas (`go run ./cmd/size-lint`)
- [ ] Toda query nueva scopea por `project_id` explícito además de RLS
- [ ] Ninguna tool nueva puede devolver datos de otro proyecto
- [ ] Sin secretos ni rutas de prod hardcodeadas
- [ ] Sin N+1: el reporte por marco resuelve el crosswalk con JOIN, no con query por control
- [ ] `go test ./... -count=1` verde + integración con testcontainers
- [ ] Sabotaje ejecutado y restaurado: quitar la policy de `project_control_status`, poner el
      catálogo bajo RLS, y marcar `iso-27001` como `texto_libre`

## Documentación

- [ ] CHANGELOG Unreleased
- [ ] `state.yaml` a `implemented`
