# Proposal: roles y pertenencia de proyectos

**Esfuerzo:** XL (6 issues, entregables por separado, en dos fases que NO se solapan)
**Prioridad:** media — no hay urgencia operativa hoy, y esa es justamente la razón para hacerlo bien

## Intention

Que un proyecto tenga dueño y miembros, y que cada miembro pueda hacer dentro de él solo lo que su
rol permite. Hoy no existe ninguna de las dos cosas: cualquier credencial válida ve y toca los 25
proyectos por igual.

## El bloqueante medido

No es que el modelo de roles esté incompleto. Es que **no existe**, y lo que parece existir engaña:

| Pieza | Estado real (medido en prod, 2026-08-07) |
|---|---|
| `users.role` | varchar libre. 1 fila, valor `owner` |
| tabla `roles` | 5 filas (admin, developer, pm, qa, viewer). **`owner` no está** |
| `user_roles` / `auth_sessions` | **0 filas**. La infra de la migración 118 nunca se usó |
| `Principal.Role` | se lee en **un** lugar del código (`health_tools.go:87`), para imprimirlo |
| pertenencia de proyectos | **no existe**: ni `owner_id`, ni tabla de miembros |
| RLS de `projects` y `users` | `FORCE` con `ENABLE` en `f` → **inactivo**. Postgres ignora el force sin el enable |

Las policies RLS que quedaron inertes filtran por `organization_id = current_org_id()`, una columna
que **ya no existe**: el multi-tenancy se desmanteló a propósito en REQ-21.6 (commit `091c4cd5`).
Reactivarlas no es descomentar una línea, es reescribirlas.

## Lo que ya existe y se reusa

Tres piezas hacen que esto sea mucho más barato de lo que aparenta, y las tres están probadas:

1. **`ResilientWrapper` envuelve el 100% de las tools** y ya hace autorización por nombre vía
   `SetAllowedToolsAccessor` (`server.go:167`, DOMAINSERV-85). No hay que tocar 181 tools: hay que
   extender un wrapper que ya las intercepta a todas.
2. **`withProjectTxHandler`** (`wireup.go:175`) ya resuelve el slug, setea el GUC y **falla
   cerrado**. El patrón correcto existe — solo que cubre **8 de 181** tools.
3. **`tool_channels.go`** clasifica 166 tools con un guard (`TestAllToolsHaveChannel`) que **rompe
   CI** si una tool nueva queda sin clasificar. Es el molde exacto para la matriz tool→nivel.

## Scope

**Entra:**
- `projects.owner_id` y `projects.visibility` (`personal` | `shared`).
- `project_members(project_id, user_id, role)` con `admin | manager | developer`.
- Jerarquía por nivel y la regla única de gestión de miembros.
- Matriz tool→nivel requerido, con guard de cobertura.
- Enforcement en el wrapper que ya intercepta todas las tools.
- Tools de administración de miembros.
- RLS por pertenencia, **como fase 2 y solo al final**.

**No entra:**
- Reintroducir organizaciones o multi-tenancy. Se desmanteló a propósito.
- Permisos granulares por acción (`tickets:write`, `projects:read`). El modelo es por nivel, no por
  lista de permisos: un entero y una comparación.
- Roles custom definidos por el usuario. `custom_roles` ni siquiera existe en prod.
- Migrar `users.role` a la tabla `roles`. Los dos sistemas siguen desalineados; unificarlos es otro
  change y no bloquea a este.
- La UI de `services/domain-admin`.

## Approach

**Pertenencia primero, RLS al final.** El orden no es una preferencia de estilo, es la mitigación
del modo de falla más caro de este sistema: bajo RLS, un `SELECT` sin el GUC devuelve **cero filas
sin error**. Un filtro mal puesto en la capa de servicio produce un bug visible; el mismo error en
una policy produce datos que desaparecen en silencio.

Por eso las issues 66.1 a 66.5 entregan el modelo completo **sin tocar RLS**, con el filtro en la
capa de servicio. La 66.6 agrega el RLS como red de seguridad, recién cuando esté claro qué caminos
son globales por diseño.

## Risks

- **El RLS aplicado antes de tiempo rompe los caminos globales sin avisar**: el backfill de
  embeddings, el `/receive` público de webhooks y cualquier barrido cross-project dejan de ver
  datos. No hay excepción ni log — simplemente no hay filas.
- **La suite unitaria va a pasar con la feature rota.** Está medido en este repo: solo la de
  integración ve el RLS, y hay fixtures que no pasan `Pool`, así que la persistencia se saltea y el
  test da verde sin evaluar nada.
- **El catálogo global no puede quedar del lado del scope.** Policies de plataforma, skills globales
  y marcos de compliance tienen que seguir siendo visibles para todos: si se les aplica el filtro,
  un developer deja de ver las reglas que lo gobiernan.
- **~40 tools no tienen noción de proyecto** (`client`, `sync`, `attachment`, `cron_crud`,
  `workflow`, `error_reporting`, `mem_usage`, `health`). No se pueden filtrar por membresía sin
  decidir explícitamente qué pasa con ellas — y "se decide después" es cómo queda un agujero.
- **Un usuario borrado deja proyectos huérfanos e invisibles** si `owner_id` no tiene
  `ON DELETE RESTRICT` o reasignación forzada.
- **Con 1 solo usuario real, nada de esto se ejercita.** El primer bug de visibilidad aparece el día
  que exista el segundo usuario, no antes. Los tests tienen que crear los usuarios que la instalación
  no tiene.
