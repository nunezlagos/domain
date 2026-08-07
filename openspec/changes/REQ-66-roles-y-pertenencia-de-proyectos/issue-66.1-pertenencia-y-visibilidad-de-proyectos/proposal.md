# Proposal: pertenencia y visibilidad de proyectos

**REQ padre:** REQ-66-roles-y-pertenencia-de-proyectos
**Esfuerzo:** L — creció respecto de la primera versión: el default `personal` destapó que el slug
dejó de ser único
**Prioridad:** alta — es la fundacional; las otras cinco issues dependen de ella
**Cubre:** REQ-1, REQ-2, REQ-3 del spec

## Intention

Que un proyecto tenga dueño, un modo de visibilidad y una lista de miembros. Sin esto no hay nada
sobre lo que autorizar.

## Scope

**Entra:**
- `projects.owner_id` con `ON DELETE RESTRICT`.
- `projects.visibility` (`personal` | `shared`), **default `personal`**.
- `project_members(project_id, user_id, role)`, PK compuesta.
- **Unicidad de slug por índice parcial** — hoy no existe ninguna.
- **Arreglo de la idempotencia de `session_register`**, que hoy filtra proyectos ajenos.
- Backfill de los **25 proyectos** existentes: `owner_id` y `visibility='shared'`.
- Filtro de visibilidad en el listado, **en la capa de servicio**.

**No entra:**
- RLS (66.6).
- Autorización de acciones (66.2 a 66.4).
- Tools para administrar miembros (66.5).
- El flujo de "solicitar acceso" a un proyecto compartido ajeno: se detecta y se ofrece, pero el
  alta la resuelve 66.5.

## Approach

### El default es `personal`, y los existentes van a `shared`

Son dos decisiones distintas y conviene no confundirlas:

- **Un proyecto nuevo nace `personal`.** Compartir es un acto deliberado; no compartir no debería
  requerir ninguno. Y el riesgo de "dato que desaparece sin que nadie lo note" no aplica acá,
  porque el creador siempre ve lo que creó.
- **Los 25 existentes van a `shared`.** Se crearon cuando no había concepto de privacidad, ya tienen
  slug y —9 de ellos— repo registrado. Marcarlos `personal` los volvería invisibles para cualquier
  segundo usuario futuro sin que nadie lo haya decidido.

Dato que baja el riesgo del backfill: **hoy no puede romper nada**. El único usuario existente tiene
rol global `owner` y ve todo, con cualquier valor de `visibility`. La decisión recién importa cuando
aparezca el segundo usuario.

### El hallazgo que agrandó esta issue: el slug no es único

Medido en prod: la única constraint de `projects` es `projects_pkey PRIMARY KEY (id)`. La
`UNIQUE(organization_id, slug)` se fue junto con `organization_id` en REQ-21.6. Hoy no hay
duplicados —25 slugs, 25 únicos— pero **nada lo impide**: se sostiene solo porque hay un usuario
que no repitió nombres.

Y hay algo peor. `session_bootstrap_tools.go:665` hace la idempotencia **por slug** y devuelve el
proyecto encontrado **entero**:

```go
if existing, _ := h.projects.GetBySlug(ctx, orgID, slug); existing != nil {
    return toolResultJSON(map[string]any{"known": true, "project": existing, ...})
}
```

Con proyectos personales eso es una fuga directa: si A tiene `api` personal y B abre una carpeta
llamada `api`, **B recibe el proyecto de A completo** —descripción, repo, todo— y queda trabajando
sobre él. No es hipotético: los proyectos se auto-registran desde `session_bootstrap` cada vez que
alguien abre un directorio nuevo, y "api", "web" o "backend" son nombres que se repiten solos.

### Unicidad por índice parcial

```
UNIQUE (slug)           WHERE visibility = 'shared'     -- namespace global
UNIQUE (owner_id, slug) WHERE visibility = 'personal'   -- namespace por persona
```

Un proyecto compartido tiene nombre inequívoco; un personal solo compite consigo mismo. Postgres
soporta índices parciales, así que no hace falta ninguna estructura adicional.

### La identidad fuerte es el remoto, no el slug

El slug es un nombre; el remoto es una identidad. Dos carpetas con el mismo remoto son el mismo
proyecto aunque se llamen distinto; dos con el mismo nombre y distinto remoto **no lo son**. Es
exactamente la distinción que la policy `cross-project-context` ya pide: "nunca asumir que el
proyecto actual es el del basename del cwd".

De ahí sale la regla del auto-registro:

1. ¿Hay un proyecto **visible para mí** con ese remoto? → es ese.
2. ¿Hay un proyecto **visible para mí** con ese slug? → es ese.
3. ¿Hay un proyecto **compartido** con ese remoto del que no soy miembro? → se ofrece solicitar
   acceso; **no se entra solo**.
4. Si no → proyecto nuevo, `personal`, mío.

Un slug que existe pero es invisible se comporta como inexistente. Es la única forma de no revelar
lo que no se puede ver.

## Risks

- **`ON DELETE RESTRICT` bloquea el borrado de usuarios.** Es el objetivo, pero el flujo de borrado
  (`lifecycle/erasure.go`) tiene que dar un mensaje claro en vez de un error de constraint crudo.
- **Cambiar la idempotencia de `session_register` cambia un contrato observable.** Hoy devuelve
  `known=true` ante cualquier slug coincidente; va a devolverlo solo ante los visibles. Es el
  arreglo de una fuga, pero es un cambio de comportamiento y hay clientes que lo consumen.
- **El índice parcial no cubre el cambio de visibilidad.** Pasar un proyecto de `personal` a
  `shared` puede colisionar con un `shared` que ya use ese slug. Esa transición necesita su propia
  validación y un mensaje que explique el conflicto.
- **Un default mal elegido para el backfill es difícil de revertir**: si los 25 quedan `personal` y
  después aparece un segundo usuario, el síntoma es "no veo nada", sin ningún error.
- **Con 1 usuario, el filtro no se puede probar con datos reales.** Los tests crean su población.
