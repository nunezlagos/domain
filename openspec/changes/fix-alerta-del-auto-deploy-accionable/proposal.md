# El aviso del auto-deploy pasa a ser accionable

**EXTENSIÓN de issue-57.2** (`issue-57.2-auto-deploy-duenos-y-entorno-del-unit`,
DOMAINSERV-258, ya `implemented`). No es trabajo nuevo: son los tres hallazgos que el
propio 4R de ese issue levantó sobre el camino de la alerta y que quedaron sin abordar
al cerrarlo. Se aborda como extensión por decisión explícita del usuario, en vez de
abrir tickets nuevos.

## Why

El auto-deploy ya despliega solo — verificado el 2026-08-09: tag `v0.7.6`, ciclo del
timer 16:21:02 CEST, `ExecMainStatus=0`, HEAD del VPS en el commit del tag sin
intervención manual. Lo que seguía roto era el AVISO.

El ADR-4 de issue-57.2 justificó la unit de alerta diciendo que lee el motivo del
journal "para que el aviso sea accionable". Esa parte no se cumplía:

1. **El aviso no decía la causa.** `journalctl -n 5 | tail -n 3` se queda con las tres
   últimas líneas del journal, y en una unit que acaba de fallar las últimas son
   siempre las que emite systemd (`Main process exited`, `Failed with result`), que
   desplazan a las del script. Medido leyendo el canal con `&since=all`: ~30 avisos en
   7 horas, todos diciendo `Failed with result exit-code`, mientras la causa real
   —`validate: .env writable — abort`— estaba en `/opt/services/.deploy.log`. Para
   saber por qué fallaba hubo que entrar al VPS a leer un archivo.

2. **Un fallo permanente notificaba cada 10 minutos.** `OnFailure=` dispara una vez por
   ciclo del timer. El canal es compartido con `healthcheck-alert.sh` y con los avisos
   de backup, así que un fallo persistente —que es el modo de falla que issue-57.2 vino
   a resolver— lo convierte en ruido y la próxima alerta real se pierde entre repetidos.

3. **El cuerpo iba sin cota a un canal público.** El ADR-4 se comprometió a mandar las
   líneas RECORTADAS y el único recorte era `tail -n 3`: sin límite de longitud ni
   filtro, el contrato efectivo era "lo que el script escriba a stderr se publica en
   ntfy.sh". Hoy no filtraba nada porque `log_phase` solo emite fases y rutas, pero
   nada lo garantizaba.

## Scope

Entra: de dónde sale el motivo del fallo, el throttle por huella del motivo, y el
recorte por longitud de línea. Todo dentro de `services/scripts/auto-deploy-alert.sh`.

Queda fuera: `healthcheck-alert.sh` y el canal compartido; el auto-deploy en sí, que ya
funciona; y cualquier cambio a la política de qué se despliega.

## Approach

El motivo sale del `.deploy.log`, que es donde vive, con el journal como respaldo
descartando lo que systemd agrega. El throttle es por HUELLA DEL MOTIVO y no por el
hecho de fallar, así que una causa nueva siempre avisa. El estado vive en `/run` para
que un reboot lo olvide.

## Risks

El riesgo dominante es que el throttle silencie información: se acota haciendo la huella
del motivo y no del evento, con escenario propio que lo verifica. El segundo es que el
recorte tire la parte útil del mensaje: se acota cortando a 512 chars por línea y
verificando que el principio del motivo sobreviva.

## Testing

Tres tests con su sabotaje, sobre un espejo del repo en tmpdir. La verificación final NO
se cierra leyendo el script: se cierra mirando el canal real con `&since=all` después de
un fallo, que es como se detectaron los tres hallazgos.
