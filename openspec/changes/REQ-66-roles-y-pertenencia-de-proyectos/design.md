# Design: roles y pertenencia de proyectos

## Decisión 1 — Un entero y una comparación, no una lista de permisos

El repo ya tiene un catálogo de permisos por rol (`roles.permissions`: `tickets:write`,
`projects:read`, ...) y lleva 0 filas asignadas desde que se creó. Es la evidencia de que el modelo
granular no se sostuvo en la práctica.

El modelo es **ordinal**:

```
owner     3
admin     2
manager   1
developer 0
```

Autorizar es `nivel(actor) >= nivel_requerido(tool)`. Gestionar miembros es
`nivel(actor) > nivel(objetivo)`.

**Por qué importa para el costo:** con 181 tools, un chequeo de una línea por tool es viable; una
evaluación de listas de permisos por tool no lo es. La decisión de modelo es la que hace la
auditoría alcanzable.

**Lo que se pierde:** no se puede expresar "este rol puede X pero no Y" si X e Y están en el mismo
nivel. Se acepta: el pedido del usuario fue una cadena de mando, no un tablero de permisos.

## Decisión 2 — Dos ejes que no compiten

| Eje | Fuente | Responde |
|---|---|---|
| visibilidad | `users.role` (global) + `visibility` + membresía | qué proyectos **veo** |
| capacidad | `project_members.role` | qué **hago** dentro de uno que ya veo |

La alternativa —un solo rol que decida ambas cosas— obliga a una regla de precedencia ("gana el
mayor", "el de proyecto pisa al global") y toda regla de precedencia tiene casos borde que se
descubren en producción.

Consecuencia deliberada, ya aceptada por el usuario: **un proyecto `shared` del owner lo ve el
admin**. Si el owner no quiere eso, lo marca `personal`. Esto reemplaza la regla especial "el admin
no ve los proyectos del owner", que habría exigido codificar visibilidad como función del rol del
dueño además del rol del que mira.

## Decisión 3 — Privado por defecto, y el nombre deja de ser identidad

Un proyecto nuevo nace `personal`; los 25 existentes van a `shared`. Son dos decisiones separadas:
compartir debe ser un acto deliberado, pero lo que ya existía se creó sin concepto de privacidad y
volverlo invisible sería una decisión que nadie tomó.

Esa elección destapó un problema que estaba latente: **el slug no es único**. La constraint
`UNIQUE(organization_id, slug)` se fue junto con `organization_id`, y lo único que queda es
`projects_pkey`. Hoy no hay duplicados porque hay un solo usuario que no repitió nombres.

Y `session_register` (`session_bootstrap_tools.go:665`) hace la idempotencia **por slug**,
devolviendo el proyecto encontrado **entero**. Con proyectos personales eso deja de ser una
molestia y pasa a ser una fuga: dos personas con una carpeta `api` terminarían compartiendo
proyecto sin haberlo decidido, y la segunda recibiría los datos de la primera. Los proyectos se
auto-registran al abrir un directorio, así que el caso es cotidiano, no teórico.

La resolución tiene dos partes:

```
UNIQUE (slug)           WHERE visibility = 'shared'     -- nombre inequívoco
UNIQUE (owner_id, slug) WHERE visibility = 'personal'   -- cada uno con su namespace
```

y **la identidad fuerte pasa a ser el remoto, no el slug**. Dos carpetas con el mismo remoto son el
mismo proyecto aunque se llamen distinto; dos con el mismo nombre y distinto remoto no lo son. Es
lo que la policy `cross-project-context` ya pedía: "nunca asumir que el proyecto actual es el del
basename del cwd".

Un slug que existe pero es invisible **se comporta como inexistente**. Es la única forma de no
revelar lo que no se puede ver.

## Decisión 4 — `owner_id` es columna, no rol

Un rol `owner` dentro de `project_members` permite dos estados inválidos: cero owners y dos owners.
La columna los hace imposibles por construcción.

Hay una razón extra específica de este repo: ya existe `users.role='owner'` con un catálogo `roles`
que **no contiene** `owner`. Repetir el mismo nombre en un tercer lugar con semántica distinta es
exactamente cómo nació esa confusión.

`ON DELETE RESTRICT` sobre `owner_id`, o reasignación forzada previa. Sin eso, borrar un usuario
deja proyectos que **nadie** puede ver ni administrar — y son invisibles, así que nadie nota que
quedaron.

## Decisión 5 — El enforcement va en el wrapper, no en los handlers

Ya existen dos puntos de intercepción, medidos:

- **`ResilientWrapper`** — envuelve el **100%** de las tools y ya autoriza por nombre vía
  `SetAllowedToolsAccessor` (`server.go:167`). Es donde va el chequeo de nivel.
- **`withProjectTxHandler`** (`wireup.go:175`) — resuelve el slug, setea el GUC y **falla cerrado**.
  Cubre **8 de 181** tools; el resto usa `withOrgTxHandler` (35 usos) y resuelve el proyecto por su
  cuenta.

El trabajo no es escribir el mecanismo: es **extender la cobertura de uno que ya funciona**. Poner
el control en cada handler significa 181 oportunidades de olvidarlo, y un olvido no produce ningún
síntoma.

## Decisión 6 — La matriz tool→nivel se calca de `tool_channels.go`

`tool_channels.go` clasifica 166 tools y `TestAllToolsHaveChannel` **rompe CI** si una tool nueva
queda sin clasificar. Es la prueba de que la invariante "cero tools huérfanas" se sostiene en este
repo con 181 tools.

La matriz de niveles usa el mismo molde: mapa explícito + guard de cobertura. Descubrir el nivel por
convención de nombre (`*_delete` → admin) parece más elegante y es peor: una tool mal nombrada
quedaría mal autorizada en silencio, y la lista explícita fuerza la **decisión**.

### Las ~40 tools sin noción de proyecto

`client`, `sync`, `attachment`, `cron_crud`, `workflow`, `error_reporting`, `mem_usage`, `health` no
reciben `project_slug`. No se pueden filtrar por membresía. Cada una necesita una decisión explícita
entre tres:

1. **global-lectura** — cualquiera autenticado (ej. `health`).
2. **global-admin** — exige rol global `admin` u `owner` (ej. `cron_crud`, `client`).
3. **necesita eje de proyecto** — hay que agregárselo antes de poder autorizarla.

Dejarlas sin clasificar es el agujero que el guard existe para impedir.

## Decisión 7 — RLS al final, y solo sobre ejes ya probados

El modo de falla más caro de este sistema está medido: bajo RLS, un `SELECT` sin el GUC devuelve
**cero filas sin error**. No hay excepción, no hay log, no hay diferencia observable con "no hay
datos".

Por eso el orden es pertenencia → servicio → RLS, y no al revés:

- un filtro mal puesto en la capa de servicio produce un bug **visible**;
- la misma equivocación en una policy produce datos que **desaparecen**.

Además, las policies inertes de `projects` filtran por `organization_id = current_org_id()` — una
columna que ya no existe. No se "reactivan": se reescriben desde cero contra el eje nuevo.

### Caminos que el RLS rompería si se aplicara sin cuidado

- backfill de embeddings (barrido global por diseño),
- `/receive` de webhooks (endpoint **público**, solo conoce el slug),
- cualquier reporte cross-project,
- el catálogo global: policies de plataforma, skills globales, marcos de compliance. Si se les
  aplica el filtro, **un developer deja de ver las reglas que lo gobiernan**.

## Decisión 8 — Los tests tienen que fabricar la población que no existe

La instalación tiene **1 usuario** y **25 proyectos**. Ningún test que use los datos reales puede
detectar un fallo de aislamiento: con un solo sujeto, todo modelo de permisos parece correcto.

Cada issue crea sus usuarios, membresías y proyectos. Y el sabotaje obligatorio no es cosmético
acá — el bug típico de este dominio (un `WHERE` que sobra, un `JOIN` que trae de más) deja la suite
en verde si los fixtures tienen un solo actor.

Riesgo conocido y medido del repo: hay fixtures que **no pasan `Pool`**, así que la persistencia se
saltea en silencio y el test pasa sin evaluar nada. Los tests de este REQ tienen que verificar que
realmente tocaron la base.

## Alternativas descartadas

**Reintroducir organizaciones.** Daría aislamiento gratis vía el RLS que ya existía. Se descarta: se
desmanteló a propósito en REQ-21.6, la instalación es mono-tenant y volver atrás es más caro que el
modelo nuevo — habría que restaurar la tabla, su columna en 37 tablas y todas las policies.

**Roles custom por proyecto.** `custom_roles` existe como migración y no existe en prod. Es
justamente el over-engineering que la policy YAGNI señala: 1 usuario, ningún caso que lo pida.

**Permisos por acción en vez de niveles.** Ya está en el repo (`roles.permissions`), con 0 filas
asignadas. Cuesta más y no se usó.

**Autorización nominal manager→developer** (cada developer reporta a un manager concreto). Abre
preguntas sin respuesta obvia —developer con dos managers, qué pasa con las asignaciones si el
manager se va, si un manager ve el trabajo de developers de otro— y no hay ningún caso real que la
exija. Se hace **por nivel**; si algún día hace falta, se agrega una columna sin rehacer nada.
