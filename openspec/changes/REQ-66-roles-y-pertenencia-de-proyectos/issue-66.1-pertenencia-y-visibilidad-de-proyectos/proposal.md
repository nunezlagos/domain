# Proposal: pertenencia y visibilidad de proyectos

**REQ padre:** REQ-66-roles-y-pertenencia-de-proyectos
**Esfuerzo:** M
**Prioridad:** alta — es la fundacional; las otras cinco issues dependen de ella
**Cubre:** REQ-1, REQ-2, REQ-3 del spec

## Intention

Que un proyecto tenga dueño, un modo de visibilidad y una lista de miembros. Sin esto no hay nada
sobre lo que autorizar.

## Scope

**Entra:**
- `projects.owner_id` con `ON DELETE RESTRICT`.
- `projects.visibility` (`personal` | `shared`), con default explícito.
- `project_members(project_id, user_id, role)`, PK compuesta.
- Backfill de los **25 proyectos** existentes: todos necesitan `owner_id`.
- Filtro de visibilidad en el listado de proyectos, **en la capa de servicio**.

**No entra:**
- RLS (es la 66.6).
- Autorización de acciones (66.2 a 66.4). Acá solo se decide **qué se ve**.
- Tools para administrar miembros (66.5). El backfill se hace por migración.

## Approach

El default de `visibility` es la decisión con más consecuencias de esta issue: define qué pasa con
los 25 proyectos existentes y con cada proyecto nuevo.

Se propone **`shared` como default**, con este razonamiento: los 25 proyectos actuales fueron
creados cuando no existía el concepto de privacidad, en una instalación de un solo usuario. Marcarlos
`personal` los volvería invisibles para cualquier segundo usuario futuro sin que nadie lo haya
decidido — y como son invisibles, nadie notaría el error. Un default `shared` falla del lado
visible.

El dueño del backfill es el único usuario existente. Si en el futuro hubiera más de uno al momento
de migrar, la migración debe **fallar** en vez de elegir por su cuenta.

## Risks

- **`ON DELETE RESTRICT` bloquea el borrado de usuarios.** Es el objetivo, pero hay que verificar que
  el flujo de borrado (`lifecycle/erasure.go`) lo maneje con un mensaje claro en vez de un error de
  constraint crudo.
- **El filtro de listado se aplica en un solo lugar hoy, pero puede haber otros caminos** que
  devuelvan proyectos (bootstrap de sesión, búsqueda global). Todos tienen que pasar por el mismo
  filtro o el aislamiento tiene un agujero por donde nadie mira.
- **Un default mal elegido es difícil de revertir**: si los 25 quedan `personal` y después aparece
  un segundo usuario, el síntoma es "no veo nada" sin ningún error.
- **Con 1 usuario, el filtro no se puede probar con datos reales.** Los tests crean su propia
  población.
