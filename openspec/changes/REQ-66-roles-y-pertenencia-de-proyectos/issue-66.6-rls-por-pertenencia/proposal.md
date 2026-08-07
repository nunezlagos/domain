# Proposal: RLS por pertenencia (fase 2)

**REQ padre:** REQ-66-roles-y-pertenencia-de-proyectos
**Esfuerzo:** L
**Prioridad:** baja — **y esa prioridad es parte del diseño, no una omisión**
**Cubre:** REQ-7 del spec
**Depende de:** 66.1 a 66.4 implementadas **y en uso**

## Intention

Que el aislamiento por pertenencia se sostenga también si un filtro de la capa de servicio se
olvida. Defensa en profundidad, no el mecanismo primario.

## Scope

**Entra:**
- Policies nuevas sobre `projects` y las tablas cuyo eje de scope ya esté probado.
- `ENABLE` además de `FORCE` — hoy 16 tablas tienen solo el force y **no están protegidas**.
- Inventario de caminos globales, con su exención explícita.
- Suite de integración multi-usuario.

**No entra:**
- Las 37 tablas con `project_id` de una sola vez. Se va tabla por tabla, en orden de riesgo.
- Reintroducir `organization_id` ni sus policies viejas.

## Approach

**Esta issue va última, y el orden es la mitigación principal.** Bajo RLS, un `SELECT` sin el GUC
devuelve **cero filas sin error**: no hay excepción, no hay log, no hay diferencia observable con
"no hay datos". Un filtro mal puesto en la capa de servicio produce un bug visible; la misma
equivocación en una policy produce datos que desaparecen.

Por eso el RLS entra cuando el eje de pertenencia ya está probado en producción por 66.1-66.4, no
antes.

### Lo que hay que limpiar primero

- **16 tablas tienen `FORCE` con `ENABLE` en `f`**: parecen protegidas y no lo están, porque
  Postgres ignora el force sin el enable. Entre ellas `projects` y `users`.
- **Las policies inertes de `projects` filtran por `organization_id = current_org_id()`**, una
  columna que ya no existe. No se reactivan: se reescriben.

Dejar esas policias muertas en el schema es peor que no tenerlas: cualquiera que mire `pg_class` o
`pg_policies` va a concluir que hay aislamiento donde no lo hay.

### Caminos globales que hay que eximir explícitamente

- backfill de embeddings — barrido global por diseño,
- `/receive` de webhooks — endpoint **público**, solo conoce el slug,
- reportes cross-project,
- el catálogo global: policies de plataforma, skills globales, marcos de compliance.

## Risks

- **El modo de falla es silencioso por naturaleza.** Es la razón entera de que esta issue sea la
  última. Cada tabla que se sume necesita su verificación de que los caminos globales siguen viendo
  lo suyo.
- **La suite unitaria va a pasar con el RLS roto.** Está medido en este repo: solo la de integración
  lo ve, y hay fixtures que no pasan `Pool` y saltean la persistencia sin decirlo.
- **Con 1 usuario real no hay forma de detectar una fuga**: los tests tienen que fabricar varios
  usuarios con distinta pertenencia. Un test con un solo actor da verde contra cualquier policy,
  incluso una que no filtre nada.
- **`current_project_id()` ya existe y funciona** para 9 tablas. Extender el patrón es más seguro
  que inventar uno nuevo, pero el GUC es LOCAL a la transacción: fuera de una tx no sobrevive, y ahí
  el RLS devuelve cero filas.
- **Una policy nueva puede romper un flujo que hoy anda y que nadie mira** — un cron, un webhook,
  un barrido nocturno. El inventario de caminos globales no es opcional.
