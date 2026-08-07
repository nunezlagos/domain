# Proposal: jerarquía de roles por nivel

**REQ padre:** REQ-66-roles-y-pertenencia-de-proyectos
**Esfuerzo:** S — la más chica del REQ, y la que más decisiones congela
**Prioridad:** alta
**Cubre:** REQ-4 del spec

## Intention

Convertir la cadena de mando en una **función pura**: dos enteros y una comparación. Toda la
autorización del REQ se apoya en ella.

## Scope

**Entra:**
- Los cuatro niveles: `owner` 3, `admin` 2, `manager` 1, `developer` 0.
- `PuedeAdministrar(actor, objetivo) bool` → `nivel(actor) > nivel(objetivo)`.
- `TieneNivel(actor, requerido) bool` → `nivel(actor) >= requerido`.
- Parseo de un rol desconocido: qué pasa con un valor que no está en la escala.

**No entra:**
- Persistencia. Esta issue no toca la base.
- La matriz tool→nivel (66.3) ni su aplicación (66.4).

## Approach

Función pura, sin base de datos, por la misma razón que la comparación de versiones de REQ-57 se
sacó del hook: es la única pieza que **decide**, y meterla adentro de un handler la dejaría solo
testeable levantando el servidor entero.

El caso que define la calidad de esta issue es el **rol desconocido**. Hoy `users.role` es un
varchar libre y ya contiene un valor (`owner`) que la tabla `roles` no tiene. O sea: los datos
existentes ya prueban que van a llegar valores fuera de la escala.

Un rol que no se puede ubicar **no es nivel 0**: nivel 0 significa "developer", y un developer puede
hacer cosas. Un valor irreconocible tiene que ser **rechazo**, no el permiso más bajo. Confundirlos
convierte un typo en un acceso.

## Risks

- **Tratar lo desconocido como el nivel más bajo** parece seguro y no lo es: `developer` puede
  operar. Lo desconocido no debe poder nada.
- **La comparación con `>` vs `>=` en gestión de miembros** es la diferencia entre "un admin puede
  quitar a otro admin" y "no puede". Es un solo carácter y cambia el modelo — necesita su propio test.
- **La escala se puede quedar corta** si aparece un rol nuevo. Los valores numéricos deben tener
  espacio entre sí o el orden se vuelve frágil.
