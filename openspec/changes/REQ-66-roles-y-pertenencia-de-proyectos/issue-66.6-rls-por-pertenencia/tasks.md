# Tasks: RLS por pertenencia (fase 2)

## Precondición — NO empezar sin esto

- [ ] 66.1 a 66.4 implementadas Y en uso, no solo mergeadas
- [ ] Al menos 2 usuarios reales en la instalación
  — con uno solo, cualquier policy da verde, incluso una que no filtre nada

## Limpieza previa

- [ ] Inventariar las 16 tablas con `FORCE` y `ENABLE` en `f`: decidir por cada una si se activa o
      se le quita el force
  — dejarlas así es peor que no tenerlas: quien mire `pg_class` va a concluir que hay aislamiento
- [ ] Dropear las policies inertes de `projects` que filtran por `organization_id`
  — la columna ya no existe; no se reactivan, se reescriben
- [ ] Inventario escrito de caminos globales: backfill de embeddings, `/receive` de webhooks,
      reportes cross-project, catálogo global

## Tests (RED primero)

- [ ] Test de integración con 3+ usuarios de distinta pertenencia: cada uno ve exactamente lo suyo
- [ ] Test: un barrido cross-project procesa todas sus filas con el RLS activo (REQ-7)
- [ ] Test: una consulta sin el GUC FALLA en vez de devolver cero filas (REQ-7)
- [ ] Test: el catálogo global sigue siendo legible por un developer
- [ ] Test: el endpoint público de webhooks sigue resolviendo por slug

## Código

- [ ] Policy nueva sobre `projects` contra el eje de pertenencia
- [ ] `ENABLE` además de `FORCE` en cada tabla que se sume
- [ ] Ir tabla por tabla en orden de riesgo, NO las 37 de una vez
- [ ] Exención explícita y documentada por cada camino global

## Verify (auditoría — última task)

- [ ] Suite de integración verde — la unitaria NO alcanza y no cuenta como evidencia acá
- [ ] Sabotaje: quitar el GUC de una ruta → su test en rojo con error, no con cero filas
- [ ] Sabotaje: dejar una tabla con FORCE sin ENABLE → el guard de la limpieza previa en rojo
- [ ] Verificar el RLS EFECTIVO contra `pg_class` (`relrowsecurity` Y `relforcerowsecurity`), no
      contra los archivos de migración
- [ ] Correr los caminos globales contra el ambiente real y confirmar que devuelven datos
- [ ] `db-conventions-lint` verde

## Documentación

- [ ] CHANGELOG Unreleased
- [ ] `state.yaml` a `implemented`
- [ ] Documentar qué tablas quedaron bajo RLS y cuáles no, con la razón
