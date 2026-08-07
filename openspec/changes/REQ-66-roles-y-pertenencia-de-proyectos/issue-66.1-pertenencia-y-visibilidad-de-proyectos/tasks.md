# Tasks: pertenencia y visibilidad de proyectos

## Schema

- [ ] Migración: `projects.owner_id UUID REFERENCES users(id) ON DELETE RESTRICT`
- [ ] Migración: `projects.visibility TEXT NOT NULL DEFAULT 'personal'` con CHECK de los dos valores
  — un proyecto nuevo nace personal; compartir es un acto deliberado
- [ ] Migración: `project_members(project_id, user_id, role)` con PK compuesta y FK en cascada
  — el índice por `user_id` es obligatorio: el listado filtra por usuario, no por proyecto
- [ ] Migración: índices parciales de unicidad de slug, con `CONCURRENTLY`
  - [ ] `UNIQUE (slug) WHERE visibility = 'shared'`
  - [ ] `UNIQUE (owner_id, slug) WHERE visibility = 'personal'`
  — hoy NO existe ninguna unicidad de slug: la única constraint de projects es la PK por id
- [ ] Header de migración completo (migration, author, issue, description, breaking, duration)
- [ ] Backfill de los 25 proyectos: `owner_id` del único usuario y `visibility='shared'`
  — ya tienen slug y 9 tienen repo; marcarlos personal los volvería invisibles para un segundo
    usuario futuro sin que nadie lo decida
- [ ] La migración FALLA si hay más de un usuario al correr el backfill
  — elegir dueño por su cuenta sería inventar una decisión que nadie tomó

## Tests (RED primero)

- [ ] Test: un proyecto nuevo queda con `owner_id` sin paso adicional (REQ-1)
- [ ] Test: un proyecto nuevo queda `personal` (REQ-2)
- [ ] Test: borrar un usuario dueño de un proyecto es rechazado con error explícito (REQ-1)
- [ ] Test: un proyecto `personal` no aparece para otro usuario, cualquiera sea su rol global (REQ-2)
- [ ] Test: el rol global `owner` sí ve los proyectos personales ajenos (REQ-2)
- [ ] Test: dos usuarios pueden tener cada uno un `personal` con el mismo slug (REQ-2)
- [ ] Test: dos `shared` con el mismo slug son rechazados (REQ-2)
- [ ] Test: registrar un slug que existe como personal AJENO crea uno nuevo y propio (REQ-2)
  — y NO devuelve ni un solo campo del proyecto ajeno: es el test de la fuga
- [ ] Test: un developer ve exactamente los compartidos donde es miembro más los suyos (REQ-3)
- [ ] Test: un admin ve los compartidos sin ser miembro (REQ-3)
- [ ] Test de migración: los 25 proyectos quedan con `owner_id` no nulo y `visibility='shared'`

## Código

- [ ] Filtro de visibilidad en el listado de proyectos, en la capa de servicio
- [ ] Inventariar TODOS los caminos que devuelven proyectos (listado, bootstrap de sesión, búsqueda
      global) y hacerlos pasar por el mismo filtro
  — un camino que se saltee el filtro es un agujero por donde nadie mira
- [ ] `session_register`: la idempotencia resuelve SOLO entre lo visible para quien pide
  — hoy `session_bootstrap_tools.go:665` devuelve el proyecto ajeno ENTERO ante cualquier slug
    coincidente
- [ ] Resolución del auto-registro en orden: remoto visible → slug visible → compartido con ese
      remoto (ofrecer acceso, NO entrar solo) → crear nuevo personal
- [ ] Validar la transición `personal` → `shared`: puede colisionar con un `shared` existente y el
      mensaje tiene que explicar el conflicto
- [ ] Mensaje claro en el flujo de borrado de usuarios cuando el RESTRICT dispara

## Verify (auditoría — última task)

- [ ] Ninguna función nueva > 50 líneas (`go run ./cmd/size-lint`)
- [ ] `go test ./... -count=1` verde
- [ ] Suite de integración: verifica que los tests REALMENTE tocan la base
  — hay fixtures en este repo que no pasan `Pool` y saltean la persistencia en silencio
- [ ] Sabotaje: quitar el filtro de visibilidad → los tests de REQ-2 y REQ-3 en rojo
- [ ] Sabotaje: volver la idempotencia de `session_register` a resolver por slug global → el test
      de la fuga en rojo
- [ ] Sabotaje: quitar el `ON DELETE RESTRICT` → el test de borrado en rojo
- [ ] `db-conventions-lint` en verde sobre las migraciones nuevas
- [ ] Verificado contra el schema EFECTIVO (`pg_constraint`, `pg_indexes`), no contra el archivo de
      migración

## Documentación

- [ ] CHANGELOG Unreleased
- [ ] `state.yaml` a `implemented`
- [ ] Documentar el cambio de contrato de `session_register`: pasa a devolver `known=true` solo
      ante proyectos visibles
