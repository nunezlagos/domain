# Proposal: administración de miembros

**REQ padre:** REQ-66-roles-y-pertenencia-de-proyectos
**Esfuerzo:** M
**Prioridad:** media — el modelo funciona sin esto, pero solo se puede administrar por SQL
**Cubre:** REQ-4 del spec (la parte operable)
**Depende de:** 66.1 (tabla de miembros), 66.2 (la regla de nivel)

## Intention

Que la cadena de mando se pueda ejercer desde el producto: designar, quitar y cambiar el rol de un
miembro, con la regla de nivel aplicada.

## Scope

**Entra:**
- Tools para agregar, quitar y cambiar el rol de un miembro.
- Tool de listado de miembros de un proyecto.
- Aplicación de `PuedeAdministrar` en las tres operaciones de escritura.
- Registro en audit de cada cambio de membresía.

**No entra:**
- Invitaciones o alta de usuarios nuevos. Eso es enrollment y ya existe.
- Transferir el `owner_id` de un proyecto. Merece su propia issue: es la única operación que puede
  dejar a alguien sin acceso a lo que creó.
- UI en `services/domain-admin`.

## Approach

Cuatro tools, mismo patrón que el resto del registry. La regla de nivel no se reimplementa: se
invoca la función pura de 66.2, y estas tools son su primer consumidor real.

**Cada cambio de membresía va a audit.** Es la operación que altera quién ve qué; sin registro, una
pérdida de acceso no tiene forma de explicarse después. Y por la policy de secretos, lo que se
registra son identificadores, nunca emails.

### El caso que hay que resolver explícitamente

Quitar al último miembro con nivel alto de un proyecto compartido lo deja sin nadie que pueda
administrarlo, salvo el `owner` global. No es un estado corrupto —el owner siempre puede entrar—
pero es un estado en el que el equipo se queda trabado sin entender por qué. La operación debería
advertirlo.

## Risks

- **Un usuario podría escalarse a sí mismo** si la regla se aplica contra el rol objetivo en vez del
  actual. El test de "nadie se auto-promueve" es obligatorio, no opcional.
- **Quitar un miembro no revoca sus credenciales**: las API keys siguen siendo válidas y su próximo
  llamado tiene que fallar por el chequeo de 66.4, no por la credencial. Si el enforcement de 66.4
  no está, esta issue da una falsa sensación de control.
- **Una condición de carrera entre dos administradores** puede dejar el proyecto sin ningún admin si
  ambos se quitan mutuamente en paralelo. La regla de nivel lo impide entre iguales, pero conviene
  verificarlo.
- **El audit no puede filtrar PII**: identificadores, nunca emails, según la policy de secretos.
