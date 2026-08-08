# El auto-deploy corre de verdad, avisa cuando falla, y el instalador subsana lo que lo rompe

DOMAINSERV-258. REQ-57, issue 57.2.

## Why

`domain-auto-deploy.service` está en `failed` y falla en CADA corrida del timer desde que existe: nunca desplegó nada y nunca avisó. Muere con `fatal: detected dubious ownership in repository at '/opt/services'` y exit 128 — aun cuando `safe.directory=/opt/services` SÍ está registrado en `/root/.gitconfig`. El unit no declara `User=`, systemd no le define `HOME`, y git nunca lee esa config. La excepción es correcta e inalcanzable por el camino real.

Es el mismo patrón que DOMAINSERV-256, con la degradación invertida: aquel falla ABIERTO, este falla CERRADO. Los dos en silencio.

Debajo hay una deuda de julio: `/opt/services` tiene los dueños mezclados (2016 archivos de root dentro de `.git`, 3363 en todo el repo, con los dirs raíz de sysadmin). Está roto en los dos sentidos — root no lee su config, y sysadmin no puede crear `refs/remotes/origin/develop.lock`. El estado ya estaba registrado el 2026-07-10 y se resolvió con un workaround (bundles + `docker run alpine/git`), nunca con un fix. `install.sh` marca `safe.directory` pero NUNCA normaliza dueños: cada corrida con sudo agrava la mezcla en vez de repararla.

## Scope

Entra: el entorno del unit, la normalización idempotente de propiedad en `install.sh`, la alerta ante fallo del ciclo reusando el camino ntfy de `healthcheck-alert.sh`, y los tests que impiden que el fix primario quede enmascarado.

Queda fuera: la política de qué se despliega, `deploy.sh` y sus 5 fases, los otros tres units, y el deploy por bundle.

## Approach

El estado se REPARA en cada corrida del instalador, no se convive con él: es la inversión que pidió el usuario. El unit deja de depender de un `HOME` que systemd no define. Ambas capas se implementan, pero el design designa cuál es PRIMARIA y cuál es defensa en profundidad, y un test falla si se quita la primaria — si se tapan mutuamente, el bug vuelve con la suite en verde.

El `chown` es la parte peligrosa: aborta si `INSTALL_DIR` viene vacío, preserva `.env` en 600, y no rompe bind mounts ni uid internos de containers.

## Risks

El riesgo dominante es un `chown -R` mal acotado sobre producción, irreversible y capaz de tirar el stack; se mitiga con la guarda de ruta vacía y la verificación de que los 5 servicios quedan healthy. El segundo es enmascarar el fix primario con el secundario, cubierto por MUST-6. El tercero es filtrar secretos por el canal de alerta, cubierto por el tercer escenario de MUST-7.

## Testing

La verificación NO se cierra leyendo el unit ni el script: se cierra con `systemctl` y `journalctl` del VPS. El MUST-5 exige ejecutar bajo systemd, porque invocar el `.sh` desde una shell hereda `HOME` y enmascara el bug — un falso verde por construcción.

## Decisiones del usuario (2026-08-08)

1. **Se repara, no se apaga.** Revierte de hecho la decisión del 2026-07-27 "CI sí, CD no: el deploy sigue siendo MANUAL" (obs `3718963a`): a partir de acá hay CD real sobre el último tag `v*`.
2. **La alerta entra al alcance.** Se propuso dejarla fuera y el usuario la incorporó: un mecanismo que falló 4 veces seguidas sin avisar no está arreglado si sigue sin avisar.
3. **El dueño destino se decide en el design**, con un ADR que mida bind mounts y uid de containers.
4. **Non-goals adoptados por asunción**: se propusieron y no hubo respuesta.

## Premisa aún NO medida

Que la causa de que git no lea `/root/.gitconfig` sea específicamente la ausencia de `HOME`. Se discrimina con `sudo env -u HOME git -C /opt/services status` contra `sudo git -C /opt/services status`. Si no reproduce, el design se reescribe sobre otra causa. No avanzar a `sdd-apply` sin cerrarla.
