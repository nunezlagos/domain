# Proposal: versión por tag y aviso de actualización

**REQ padre:** REQ-57-sincronizacion-de-versiones
**Esfuerzo:** L (1+ semana, en 6 fases entregables por separado)
**Prioridad:** alta

## Intention

Que un tag `v*` alcance para que el VPS se despliegue solo y para que cualquier usuario —incluido
uno que solo clonó la solución y nunca se logueó en Git— vea al iniciar sesión que su MCP quedó
viejo, con un único comando para ponerse al día.

## Scope

**Entra:**
- Versión inyectada con `-X` en el binario del usuario y expuesta en `domain_session_bootstrap`.
- Aviso no bloqueante en el hook `SessionStart`.
- Instalador que baja el binario de Releases (anónimo) en vez de clonar y compilar con Go.
- Comando de actualización en un paso.
- Versión mínima de cliente soportada, declarada por el server.
- Cron en el VPS que despliega ante un tag nuevo, reusando `scripts/deploy.sh`.

**No entra:**
- Auto-actualización silenciosa del cliente. Decisión explícita del usuario.
- Runner de GitHub Actions para deployar: `deploy.yml` queda manual por `workflow_dispatch`.
- El webhook inbound como disparador del deploy (evaluado y descartado en el design).
- Rollback de versión del lado del cliente.

## Approach

Reusar las tres piezas que ya existen —`release-installer.yml`, `scripts/deploy.sh` y el hook
`SessionStart`— y agregar lo único que falta: el número de versión que las une.

Orden: **detección primero, automatización después**. Las fases 0-1 dan el 80% del valor (saber que
estás desactualizado) y son casi gratis; el auto-deploy va después, porque automatizarlo antes de
tener detección acelera la divergencia sin red que la contenga.

## Risks

- **La divergencia se acelera** con deploy automático + cliente manual. Por eso la versión mínima
  soportada (REQ-6) es obligatoria, no opcional.
- **Un cron que despliega solo puede sorprender**: se acota a tags, con el rollback que `deploy.sh`
  ya trae y sin ciclos solapados.
- **El aviso se vuelve ruido** si nadie actualiza: el tono escala solo por debajo del mínimo.
- **Binario de release corrupto**: no se reemplaza el anterior hasta verificar el nuevo.
