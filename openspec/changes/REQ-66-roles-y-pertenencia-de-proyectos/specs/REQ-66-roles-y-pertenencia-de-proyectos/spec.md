# Spec: roles y pertenencia de proyectos

## Contexto

Un proyecto no tiene dueño ni miembros, y ninguna credencial válida está limitada a un subconjunto
de los 25 proyectos. Este spec define QUÉ debe cumplirse, no CÓMO.

### Bloqueante medido

`users.role` es un varchar con 1 fila (`owner`); la tabla `roles` no contiene `owner`; `user_roles`
y `auth_sessions` tienen 0 filas; y `Principal.Role` se lee en un solo punto del código para
imprimirlo. No hay autorización por rol en ninguna ruta.

`projects` y `users` tienen `FORCE ROW LEVEL SECURITY` con `ENABLE` en `f`: **parecen protegidas y
no lo están**. Sus policies inertes filtran por `organization_id`, columna que ya no existe.

### Lo que ya existe y se reusa

- `ResilientWrapper` intercepta el 100% de las tools y ya autoriza por nombre.
- `withProjectTxHandler` resuelve slug + GUC y falla cerrado (cubre 8 de 181 tools).
- `tool_channels.go` + `TestAllToolsHaveChannel`: matriz por tool con guard que rompe CI.

---

### REQ-1 — Todo proyecto MUST tener exactamente un dueño

`projects.owner_id` es una columna, no un rol en la tabla de miembros: una columna garantiza
exactamente uno, mientras que un rol admite cero o dos. Borrar al dueño no puede dejar el proyecto
sin quién lo administre.

#### Scenario: Un proyecto nuevo queda con dueño
- **Given** un usuario autenticado
- **When** crea un proyecto
- **Then** `owner_id` queda con su identificador, sin paso adicional

#### Scenario: Los proyectos existentes reciben dueño al migrar
- **Given** los 25 proyectos actuales, creados antes de que existiera `owner_id`
- **When** corre la migración
- **Then** los 25 quedan con `owner_id` no nulo
- **And** ninguno queda huérfano

#### Scenario: Borrar al dueño no deja el proyecto sin administrador
- **Given** un usuario que es dueño de al menos un proyecto
- **When** se intenta borrarlo sin reasignar
- **Then** la operación es rechazada con un error explícito
- **And** el proyecto sigue teniendo dueño

---

### REQ-2 — Un proyecto personal MUST ser invisible para todos menos su dueño

`visibility` toma `personal` o `shared`. Esta regla no admite excepción por rango: es lo que hace
que "si crean proyectos propios, los demás no los ven" sea cierto sin depender de quién pregunte.

#### Scenario: Un proyecto personal no aparece para otro usuario
- **Given** un proyecto con `visibility='personal'` cuyo dueño es el usuario A
- **When** el usuario B lista proyectos
- **Then** ese proyecto no aparece, cualquiera sea el rol global de B

#### Scenario: Ni siquiera el rango más alto de otro usuario lo alcanza
- **Given** un proyecto personal de un developer
- **When** un usuario con rol global `admin` lista proyectos o intenta abrirlo por slug
- **Then** el proyecto no aparece y el acceso directo es rechazado

#### Scenario: El dueño global sí lo ve
- **Given** un proyecto personal de cualquier usuario
- **When** el usuario con rol global `owner` lista proyectos
- **Then** el proyecto aparece

---

### REQ-3 — La visibilidad MUST depender del rol global y la pertenencia MUST decidir qué se puede hacer

Son dos ejes que no compiten: el rol global responde *qué proyectos veo*, el rol de proyecto
responde *qué puedo hacer dentro de uno que ya veo*. Sin esta separación haría falta una regla de
precedencia entre ambos, y toda regla de precedencia tiene casos borde.

#### Scenario: Un developer solo ve lo asignado y lo suyo
- **Given** un usuario con rol global `developer`
- **And** un proyecto compartido donde es miembro, otro compartido donde no lo es, y uno personal suyo
- **When** lista proyectos
- **Then** ve exactamente dos: el compartido donde es miembro y el personal suyo

#### Scenario: Un admin ve todos los compartidos sin ser miembro
- **Given** un usuario con rol global `admin`
- **And** un proyecto compartido en el que no tiene membresía
- **When** lista proyectos
- **Then** el proyecto aparece

#### Scenario: Ver un proyecto no habilita a operar en él
- **Given** un usuario con rol global `admin` que ve un proyecto compartido sin ser miembro
- **When** invoca una tool que exige nivel de proyecto
- **Then** la invocación es rechazada por falta de rol en ese proyecto

---

### REQ-4 — La gestión de miembros MUST regirse por una única regla de nivel

Los roles de proyecto están ordenados: `owner` 3, `admin` 2, `manager` 1, `developer` 0. Un actor
puede designar, quitar o cambiar el rol de otro miembro **solo si su nivel es estrictamente mayor**.
De ahí se deriva todo lo demás sin reglas adicionales.

#### Scenario: Un manager designa developers
- **Given** un usuario con rol `manager` en un proyecto
- **When** agrega a otro usuario como `developer`
- **Then** la operación es aceptada

#### Scenario: Un manager no puede designar otro manager
- **Given** un usuario con rol `manager` en un proyecto
- **When** intenta agregar a otro usuario como `manager`
- **Then** la operación es rechazada por nivel insuficiente

#### Scenario: Un admin no puede quitar a otro admin
- **Given** dos usuarios con rol `admin` en el mismo proyecto
- **When** uno intenta quitar al otro
- **Then** la operación es rechazada
- **And** solo el `owner` puede hacerlo

#### Scenario: Nadie puede escalarse a sí mismo
- **Given** un usuario con rol `developer` en un proyecto
- **When** intenta cambiar su propio rol a `manager`
- **Then** la operación es rechazada

---

### REQ-5 — Toda tool MUST declarar el nivel que exige, y ninguna puede quedar sin declarar

Una tool sin nivel declarado es un agujero silencioso: se ejecutaría sin control y nada lo delataría.
El guard de cobertura de `tool_channels.go` ya demuestra que la invariante es sostenible.

#### Scenario: Una tool nueva sin nivel rompe CI
- **Given** una tool `domain_*` agregada al registry
- **When** no se le asigna nivel requerido
- **Then** la suite falla nombrando la tool

#### Scenario: Las tools sin noción de proyecto tienen una decisión explícita
- **Given** las tools que no reciben `project_slug` (`client`, `sync`, `attachment`, `cron_crud`, `workflow`, `error_reporting`, `mem_usage`, `health`)
- **When** se revisa la matriz
- **Then** cada una está clasificada con su razón
- **And** ninguna queda como "pendiente de decidir"

#### Scenario: El catálogo global sigue siendo legible por todos
- **Given** un usuario con rol `developer`
- **When** consulta policies de plataforma, skills globales o marcos de compliance
- **Then** los recibe completos
- **And** el nivel exigido no se lo impide

---

### REQ-6 — Una invocación sin nivel suficiente MUST ser rechazada antes de tocar datos

El rechazo tiene que ocurrir en el punto que ya intercepta todas las tools, no en cada handler: un
control repetido 181 veces se olvida una vez y ese olvido no se nota.

#### Scenario: Un developer no ejecuta una tool de nivel admin
- **Given** un usuario con rol `developer` en un proyecto
- **When** invoca una tool que exige nivel `admin`
- **Then** recibe un error de autorización
- **And** la operación no llega a leer ni escribir

#### Scenario: El rechazo distingue "no podés" de "no existe"
- **Given** un usuario sin membresía en un proyecto compartido que sí puede ver
- **When** invoca una tool que exige membresía
- **Then** el error indica falta de permiso, no ausencia del proyecto

#### Scenario: Un proyecto invisible no revela su existencia
- **Given** un proyecto personal de otro usuario
- **When** alguien lo invoca por slug
- **Then** la respuesta es indistinguible de la de un proyecto inexistente

---

### REQ-7 — El aislamiento por pertenencia MUST poder verificarse en la base, sin que su ausencia rompa lo global

El RLS es defensa en profundidad, no el mecanismo primario. Se agrega al final y solo sobre las
tablas cuyo eje de scope ya esté probado en la capa de servicio.

#### Scenario: Un camino global sigue funcionando tras habilitar RLS
- **Given** el RLS por pertenencia habilitado
- **When** corre un barrido cross-project (backfill de embeddings, endpoint público de webhooks)
- **Then** procesa todas las filas que le corresponden
- **And** no devuelve un conjunto vacío en silencio

#### Scenario: Una consulta sin scope falla en vez de mentir
- **Given** una tabla bajo RLS por pertenencia
- **When** se consulta sin el GUC de proyecto seteado
- **Then** la operación falla con un error explícito
- **And** no devuelve cero filas como si no hubiera datos

#### Scenario: El aislamiento se verifica con más de un usuario
- **Given** la instalación tiene un solo usuario real
- **When** corre la suite de integración
- **Then** crea los usuarios y membresías que hacen falta
- **And** verifica que cada uno ve exactamente lo que le corresponde
