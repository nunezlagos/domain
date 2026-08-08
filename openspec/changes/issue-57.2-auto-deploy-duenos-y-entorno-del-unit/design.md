# Diseño — El auto-deploy corre de verdad, avisa cuando falla, y el instalador subsana lo que lo rompe

DOMAINSERV-258 / issue-57.2. Los 4 ADRs completos están en memoria: `38817d30`, `f2385843`, `a4979d93`, `722ac3f5`.

## La causa raíz, medida entera

| Invocación | Resultado |
|---|---|
| `sudo git -C /opt/services status` | PASA |
| `sudo env -u HOME git -C /opt/services status` | PASA |
| `sudo env -u SUDO_UID git -C /opt/services status` | PASA |
| `sudo env -u SUDO_UID -u HOME git -C /opt/services status` | **FALLA** — dubious ownership |
| `systemd-run -p EnvironmentFile=... -p WorkingDirectory=... git status` | **FALLA** — dubious ownership |
| `sudo env -u HOME git config --global --get-all safe.directory` | `fatal: $HOME not set` |
| `grep -icE '^(HOME\|XDG_\|GIT_)' /opt/services/.env` | `0` |

Hacen falta **dos condiciones simultáneas**, y el servicio cumple las dos:

1. **Sin `SUDO_UID`** → el chequeo de propiedad se dispara. Bajo `sudo`, git usa esa variable para no romperle el repo propio a quien hace `sudo git`; como `/opt/services` es de `sysadmin` y `SUDO_UID` también, coinciden. Bajo systemd: euid=0 contra dueño `sysadmin`.
2. **Sin `HOME`** → git no puede leer la excepción que lo perdonaría.

Con una sola de las dos, git pasa. Por eso toda prueba a mano da verde: `sudo` aporta las dos.

Descartado por medición: el `EnvironmentFile` no interviene.

## ADR-1 — El fix primario es el entorno del unit; el chown NO es su defensa en profundidad

La consecuencia no obvia de la tabla: **el chown a `sysadmin` NO arregla el servicio.** Corre como root, el repo seguiría siendo de `sysadmin`, y la condición 1 queda intacta. Solo un chown a **root** la eliminaría, y eso le saca a `sysadmin` la operación manual del repo.

O sea, la intuición de "normalizo dueños y se arregla todo" es falsa, y solo se ve midiendo. Un design que la asumiera dejaría el servicio roto con el change aplicado y la suite en verde.

Las dos capas quedan con propósitos **distintos**: el entorno hace que el servicio corra; la normalización hace que `sysadmin` pueda operar el repo. Eso resuelve el MUST-6 por construcción — no pueden taparse mutuamente porque no arreglan lo mismo.

## ADR-2 — `Environment=HOME=/root` en el unit

Es la opción cuyo efecto está medido: la fila 3 de la tabla es exactamente el escenario del servicio con `HOME` presente.

Rechazadas: `User=root` (expresa la intención equivocada — el servicio ya corre como root, y alguien "limpia" la redundancia y reintroduce el bug); `git -c safe.directory=` por invocación (hay git en `auto-deploy-check.sh` y en `deploy.sh`, y cada nueva invocación queda expuesta al olvido); `GIT_CONFIG_COUNT/KEY_0/VALUE_0` (la más robusta técnicamente y soportada por git 2.43.0 del VPS, pero tres variables para una idea y acopla el unit a un detalle de la API de config de git).

Tradeoff: sigue dependiendo de que `/root/.gitconfig` tenga el `safe.directory`, que `install.sh:151-154` re-registra idempotentemente en cada corrida. La alternativa `GIT_CONFIG_*` queda documentada como el upgrade natural si esa dependencia molestara.

## ADR-3 — Dueño destino `sysadmin`; el chown toca propietario y NUNCA modo

Medido: los 16 bind mounts desde `/opt/services` son archivos de config (los datos están en volumes), 15 read-only. Los containers con uid propio (`nonroot`, `nobody`, `10001`, `472`) leen por el permiso de **otros**, no por dueño. Desde la perspectiva de los containers, `sysadmin` y `root` son ambos viables — así que decide otro criterio: `sysadmin` es el usuario real, entra por SSH y está en `sudo` y `docker`.

El riesgo real no es el chown sino un `chmod` en masa "por prolijidad": restringir el bit de otros tira prometheus, loki y grafana. **El paso toca dueño y solo dueño.**

Guarda de precondición ANTES del comando: `INSTALL_DIR` vacío, relativo o `/` aborta el instalador. Un `chown -R` mal acotado sobre producción es irreversible, y el estado actual ya es una mezcla de dos usuarios sin registro de cuál archivo era de quién.

## ADR-4 — La alerta va por `OnFailure=`, no por un `trap` en el script

Un `trap ERR` **no habría capturado el fallo que motiva este change**. El exit 128 ocurre en el primer git de `decidir_y_desplegar()`, y si el fallo fuera más temprano —el `cd "$REPO_ROOT"` de la línea 30, o el `exec 9>"$LOCK_FILE"`— el trap ni existe todavía. Poner la alerta dentro del script la expone a la misma clase de fallo que debe reportar.

`OnFailure=` vive en systemd, fuera del proceso: dispara ante cualquier exit ≠ 0, timeout o señal. Costo: no puede pasar el mensaje, así que la unit de alerta lee el motivo del journal.

## Plan TDD

Todos los sabotajes se restauran por `cp` y se confirman con `sha256sum` idéntico: `git checkout --` dentro de un `.sh` evade el git-guard. Los tests de shell corren desde `scripts/tests/`, junto a la suite existente `test_auto_deploy_check.sh`.

| # | Test | Qué prueba | Sabotaje |
|---|---|---|---|
| 1 | `TestUnit_AutoDeploy_DeclaraHomeExplicito` | el unit contiene `Environment=HOME=/root` | borrar esa línea del unit |
| 2 | `TestAutoDeploy_BajoSystemd_NoFallaPorDubiousOwnership` | el ciclo corre bajo `systemd-run` con el mismo `EnvironmentFile` y `WorkingDirectory` sin `dubious ownership` | quitar `Environment=HOME=/root`: vuelve el exit 128 |
| 3 | `TestAutoDeploy_InvocadoDesdeShell_NoValidaElFix` | guard metodológico: invocar el `.sh` desde una shell pasa AUNQUE el fix no esté | ninguno — el test afirma que este camino NO discrimina, y falla si alguien lo usa como verificación |
| 4 | `TestInstall_Normaliza_DejaUnicoDueno` | tras correr, `find $INSTALL_DIR ! -user sysadmin \| wc -l` es 0 | comentar el chown |
| 5 | `TestInstall_Normaliza_EsIdempotente` | la segunda corrida no reporta cambios | hacer el chown incondicional y verboso |
| 6 | `TestInstall_InstallDirVacio_AbortaSinChown` | con `INSTALL_DIR=""` aborta antes de tocar nada | quitar la guarda de precondición |
| 7 | `TestInstall_InstallDirRelativo_AbortaSinChown` | con `INSTALL_DIR="../x"` aborta | cambiar el chequeo de absoluto por uno de no-vacío |
| 8 | `TestInstall_Normaliza_NoCambiaModos` | `.env` sigue en 600 y ningún config baja de 644 | agregar un `chmod -R` "normalizador" |
| 9 | `TestUnit_AutoDeploy_DeclaraOnFailure` | el unit referencia la unit de alerta | borrar la línea `OnFailure=` |
| 10 | `TestAlerta_SinNtfyTopic_NoRompeElCiclo` | sin `NTFY_TOPIC` el ciclo termina normal | quitar el `\|\| true` del post |
| 11 | `TestAlerta_NoFiltraElTopic` | el cuerpo del aviso no contiene el `NTFY_TOPIC` ni valores del `.env` | incluir la URL completa en el mensaje |
| 12 | `TestCapasIndependientes_QuitarUnaNoTapaLaOtra` | guard del MUST-6: quitar el entorno deja el test 2 en rojo, y quitar el chown deja el test 4 en rojo, sin interferencia cruzada | aplicar ambas capas y quitar solo una: si los dos tests siguen verdes, las capas se están tapando |

El test 3 es inusual y deliberado: codifica que **el camino de verificación equivocado no discrimina**. Es el que impide que alguien cierre el MUST-1 "verificando" desde una shell, que es como este bug sobrevivió sin que nadie lo notara.

El test 12 es un guard sobre el diseño, no sobre el código: si alguien reescribe el change de modo que las capas se solapen, el MUST-6 deja de cumplirse aunque todo lo demás siga verde.

## Nota informativa — pre-existing, NO se aborda

Los 4 units del repo comparten la forma (ninguno declara `User=` ni `Environment=`). `backup`, `healthcheck` y `domain-services` no usan git, así que hoy no les duele. Es una condición previa a este change y no empeora con él: se informa sin abrir task.
