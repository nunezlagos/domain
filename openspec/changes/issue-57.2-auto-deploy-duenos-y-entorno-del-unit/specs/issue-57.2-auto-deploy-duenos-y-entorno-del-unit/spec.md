# Spec — El auto-deploy corre de verdad, avisa cuando falla, y el instalador subsana lo que lo rompe

## ADDED Requirements

### MUST-1 — El ciclo corre sin error y despliega el último tag publicado, medido en el VPS

#### Scenario: el timer completa un ciclo sin error
- **Given** el VPS con el change desplegado
- **When** se dispara `domain-auto-deploy.service` y se consulta `systemctl is-failed domain-auto-deploy.service`
- **Then** el resultado NO es `failed`, y el `journalctl` de esa corrida no contiene `dubious ownership` ni `status=128`

#### Scenario: hay un tag v* nuevo que es punta de origin/main
- **Given** un tag `v*` publicado que coincide con la punta de `origin/main`, y un VPS cuyo HEAD es anterior
- **When** corre el ciclo del auto-deploy
- **Then** el HEAD del VPS queda en el commit de ese tag, sin intervención manual

#### Scenario: un push a main sin tag no despliega
- **Given** un commit en `origin/main` sin tag `v*`
- **When** corre el ciclo
- **Then** el HEAD del VPS no cambia — la regla vigente del script se conserva

#### Scenario: la verificación no se cierra leyendo el script
- **Given** los escenarios anteriores
- **When** se pretende darlos por cumplidos
- **Then** la evidencia es la salida real de `systemctl` y `journalctl` del VPS, no una lectura del unit ni del `.sh`

### MUST-2 — Un fallo del auto-deploy avisa, en vez de morir en silencio

#### Scenario: el ciclo falla y alguien se entera
- **Given** un `NTFY_TOPIC` configurado en el `.env` y un auto-deploy que termina con exit distinto de 0
- **When** el ciclo termina
- **Then** se emite una notificación al canal que nombra el servicio y el motivo del fallo, verificada leyendo el canal con `&since=all`

#### Scenario: sin canal configurado el ciclo no se rompe
- **Given** un `.env` sin `NTFY_TOPIC`
- **When** el ciclo falla
- **Then** el fallo se registra en el journal y el ciclo no aborta por no poder notificar — mismo degradado que `healthcheck-alert.sh`

#### Scenario: un ciclo exitoso que no despliega nada no genera ruido
- **Given** un ciclo normal sin tag nuevo
- **When** termina correctamente
- **Then** no se emite notificación — el canal solo se usa para lo excepcional

### MUST-3 — El instalador subsana el estado en cada corrida, y es idempotente

#### Scenario: un repo con dueños mezclados queda normalizado
- **Given** `/opt/services` con archivos de dos dueños distintos (el estado medido: 3363 de root)
- **When** se corre el one-liner del instalador
- **Then** al terminar, `/opt/services` tiene un dueño ÚNICO y consistente, verificable con `find /opt/services ! -user <dueño> | wc -l` devolviendo 0

#### Scenario: correrlo dos veces seguidas no cambia nada la segunda vez
- **Given** un `/opt/services` ya normalizado por una corrida previa
- **When** se corre el instalador otra vez
- **Then** el resultado es idéntico y no reporta cambios de propiedad

### MUST-4 — La normalización no destruye lo que el instalador promete preservar

#### Scenario: los secretos y datos sobreviven al chown
- **Given** un `/opt/services` con `.env` en modo 600, `certs/` y `backups/` poblados
- **When** el instalador normaliza la propiedad
- **Then** `.env` conserva modo 600, y `certs/` y `backups/` conservan su contenido — lo que `install.sh:18-20` ya promete preservar sigue preservado

#### Scenario: los servicios siguen levantando después de normalizar
- **Given** el stack corriendo
- **When** se normaliza la propiedad y se reinicia
- **Then** los 5 servicios quedan healthy: ningún bind mount ni uid interno de container se rompió

### MUST-5 — El unit no depende de un entorno que systemd no le da

#### Scenario: el servicio no hereda su configuración de un HOME implícito
- **Given** el unit versionado en `services/systemd/domain-auto-deploy.service`
- **When** systemd lo ejecuta
- **Then** el `ExecStart` obtiene la configuración de git que necesita sin depender de que systemd defina `HOME`, verificado ejecutándolo BAJO SYSTEMD — no invocando el script a mano desde una shell, que sí tiene HOME y por lo tanto enmascara el bug

### MUST-6 — El fix primario no queda enmascarado por la defensa en profundidad

#### Scenario: quitar el fix primario pone un test en rojo
- **Given** el change aplicado con sus dos capas (la del unit y la del instalador)
- **When** se desactiva la capa que el design designe como PRIMARIA
- **Then** al menos un test queda en rojo nombrando esa capa — si ambas capas se tapan mutuamente, el bug puede volver con la suite en verde, que es un antecedente registrado de este proyecto

### MUST-7 — La normalización de propiedad no afloja privilegios

#### Scenario: un usuario sin privilegios no gana acceso a los secretos por el chown
- **Given** un atacante con una cuenta sin privilegios en el VPS
- **When** el instalador normaliza la propiedad de `/opt/services`
- **Then** `.env` no queda legible para otros (modo 600 preservado), y ningún archivo del repo queda world-writable

#### Scenario: la normalización está acotada al directorio de instalación
- **Given** un `INSTALL_DIR` mal definido o vacío en el entorno
- **When** se ejecuta el paso de normalización
- **Then** el instalador aborta sin ejecutar el cambio de propiedad — un `chown -R` con la ruta vacía o en `/` es destructivo e irreversible

#### Scenario: el canal de alerta no filtra secretos
- **Given** un fallo cuyo mensaje de error incluye salida del script
- **When** se emite la notificación a ntfy
- **Then** el cuerpo no contiene credenciales del `.env` ni el propio `NTFY_TOPIC`, que ES la credencial del canal
