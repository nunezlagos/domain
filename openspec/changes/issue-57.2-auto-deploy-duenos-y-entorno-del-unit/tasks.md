# Tasks — issue-57.2-auto-deploy-duenos-y-entorno-del-unit

TDD estricto: cada bloque de tests se escribe EN ROJO antes de su implementación. Los sabotajes se restauran por `cp` y se confirman con `sha256sum` — `git checkout --` dentro de un `.sh` evade el git-guard.

## Tests

- [ ] t1 — Escribir en rojo los tests del entorno del unit: `TestUnit_AutoDeploy_DeclaraHomeExplicito`, `TestAutoDeploy_BajoSystemd_NoFallaPorDubiousOwnership` y `TestAutoDeploy_InvocadoDesdeShell_NoValidaElFix`, en `scripts/tests/`. Done cuando los 3 existen y fallan por la razón correcta, y el segundo usa `systemd-run` con los mismos `-p` del unit
- [ ] t3 — Escribir en rojo los tests de normalización: `TestInstall_Normaliza_DejaUnicoDueno`, `_EsIdempotente`, `TestInstall_InstallDirVacio_AbortaSinChown`, `_InstallDirRelativo_AbortaSinChown` y `TestInstall_Normaliza_NoCambiaModos`. Done cuando los 5 fallan por la razón correcta sobre un árbol de prueba, NUNCA sobre `/opt/services` real
- [ ] t5 — Escribir en rojo los tests de alerta: `TestUnit_AutoDeploy_DeclaraOnFailure`, `TestAlerta_SinNtfyTopic_NoRompeElCiclo` y `TestAlerta_NoFiltraElTopic`. Done cuando los 3 fallan por la razón correcta
- [ ] t8 — Escribir `TestCapasIndependientes_QuitarUnaNoTapaLaOtra`: quitar el entorno deja el test 2 en rojo y quitar el chown deja el test 4 en rojo, sin interferencia cruzada. Done cuando falla si ambas capas se tapan

## Code

- [ ] t2 — Agregar `Environment=HOME=/root` a `services/systemd/domain-auto-deploy.service` con un comentario de una línea que diga por qué (systemd no define HOME sin `User=`). Done cuando pasan t1.1 y t1.2
- [ ] t4 — Implementar la normalización de propiedad en `services/install.sh`: guarda de precondición ANTES del comando (aborta si `INSTALL_DIR` está vacío, es relativo o es `/`), luego `chown -R` al dueño destino. Cambia propietario, NUNCA modo. Done cuando pasan los 5 tests de t3 y `handleInstall` no supera el límite de tamaño
- [ ] t6 — Crear `services/systemd/domain-auto-deploy-alert.service` (oneshot) y su script, que lee el motivo del journal y postea a ntfy reusando el patrón de `services/scripts/healthcheck-alert.sh`: `${NTFY_URL:-https://ntfy.sh}/$NTFY_TOPIC` con degradado `|| true`. Done cuando pasan t5.2 y t5.3
- [ ] t7 — Agregar `OnFailure=domain-auto-deploy-alert.service` al unit del auto-deploy, y registrar la unit nueva en el paso de instalación de units de `install.sh`. Done cuando pasa t5.1 y un `systemctl list-unit-files` en el VPS la muestra

## Sabotage

- [ ] t9 — Ejecutar los 12 sabotajes de la tabla del design uno por uno, confirmando que cada test queda EN ROJO con su razón textual. Restaurar por `cp` y confirmar `sha256sum` idéntico. Done cuando los 12 están registrados con su verdict

## Docs

- [ ] t10 — Documentar en el CHANGELOG Unreleased que a partir de este change HAY CD REAL sobre el último tag `v*`, revirtiendo de hecho la decisión del 2026-07-27 "CI sí, CD no". Done cuando ningún doc del repo sigue afirmando que el deploy es exclusivamente manual

## Verify

- [ ] t11 — VERIFICACIÓN EN EL VPS: correr el instalador y disparar el timer; registrar la salida real de `systemctl is-failed`, del `journalctl` de esa corrida, y de `find /opt/services ! -user <dueño> | wc -l`. No se cierra leyendo el unit ni el script — es el antecedente del proyecto de un criterio declarado cumplido por lectura que no lo estaba
- [ ] t12 — Auditar el change completo: ningún archivo nuevo supera las 150 líneas, la guarda de ruta valida el input en el boundary, sin secretos hardcodeados (en particular el `NTFY_TOPIC` fuera del cuerpo del aviso), sin código muerto, y la suite verde contra los jobs REALES del CI
