# Proposal: enforcement en el wrapper

**REQ padre:** REQ-66-roles-y-pertenencia-de-proyectos
**Esfuerzo:** M
**Prioridad:** alta — es donde el modelo pasa de declarado a vigente
**Cubre:** REQ-6 del spec
**Depende de:** 66.1 (pertenencia), 66.2 (niveles), 66.3 (matriz)

## Intention

Que una invocación sin nivel suficiente se rechace **antes de tocar datos**, en el único punto que
ya intercepta las 181 tools.

## Scope

**Entra:**
- Chequeo de nivel en `ResilientWrapper`, junto al allowlist que ya aplica.
- Extensión de `withProjectTxHandler` a las tools que hoy resuelven el proyecto por su cuenta.
- Distinción entre "no tenés permiso" y "no existe" en los mensajes de error.

**No entra:**
- RLS (66.6).
- Las tools de administración de miembros (66.5).

## Approach

**El control va en el wrapper, no en los handlers.** `ResilientWrapper` ya envuelve el 100% de las
tools y ya autoriza por nombre vía `SetAllowedToolsAccessor` (`server.go:167`). Sumar el chequeo de
nivel ahí es un punto; repartirlo por handler son 181 oportunidades de olvidarlo — y un olvido acá
no produce ningún síntoma, solo una tool que autoriza de más.

Segundo frente: `withProjectTxHandler` (`wireup.go:175`) resuelve el slug, setea el GUC y **falla
cerrado**, pero cubre **8 de 181** tools. El resto usa `withOrgTxHandler` (35 usos) y resuelve el
proyecto por su cuenta, cada una a su manera. Migrar esas tools al wrapper de proyecto es lo que
hace que el chequeo tenga contra qué comparar.

### El mensaje de error tiene dos casos que no se pueden confundir

- **Sin permiso sobre un proyecto que sí podés ver** → decilo. Ocultarlo convierte un problema de
  permisos en un misterio de soporte.
- **Un proyecto que no podés ver** → la respuesta debe ser indistinguible de "no existe". Un mensaje
  de "no tenés permiso" sobre un proyecto personal ajeno **confirma que existe**, y eso ya es una
  fuga.

## Risks

- **Un chequeo en el wrapper aplica a todo, incluido lo que no debería.** Los caminos internos
  (hooks, agentes del orquestador, el service token del ACP) invocan tools con credenciales
  propias. Si el nivel se les aplica igual, se rompen flujos que hoy andan — y el síntoma va a
  aparecer en producción, no en la suite.
- **Migrar 35 tools de `withOrgTxHandler` a `withProjectTxHandler` cambia su comportamiento ante la
  falta de `project_slug`**: pasan a fallar cerrado. Es lo correcto, pero es un cambio de contrato
  para cualquiera que hoy las invoque sin slug.
- **El orden de los chequeos importa**: primero visibilidad (¿existe para vos?), después nivel
  (¿podés?). Invertirlo filtra la existencia de proyectos invisibles.
- **Un rechazo mal ubicado puede dejar la transacción abierta.** El wrapper ya maneja tx; el chequeo
  debe ocurrir donde el rollback esté garantizado.
