# issue-38.9-systemd-multi-service

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** media
**Tipo:** infrastructure / systemd
**Wave:** 3 (depende de 38.8)

## Historia de usuario

**Como** operador del VPS
**Quiero** que `systemd/domain-services.service` ejecute `make up` (orquestando
los 5 servicios) en el boot del VPS y `make down` en el shutdown
**Para** que después de reboot del host, todo el stack vuelva solo sin
intervención manual.

## Criterios de aceptación

### Escenario 1: Unit ejecuta make up al iniciar

```gherkin
Dado que reboot el VPS
Cuando systemd inicia
Entonces domain-services.service se activa después de docker.service
Y ExecStart=/usr/bin/make -C /opt/services up
Y los 5 containers están up dentro de 3 min
```

### Escenario 2: After + Requires correctos

```gherkin
Dado que inspecciono el unit
Cuando reviso la sección [Unit]
Entonces After=docker.service network-online.target
Y Requires=docker.service
Y Wants=network-online.target
```

### Escenario 3: ExecStop usa make down

```gherkin
Dado que ejecuto `systemctl stop domain-services`
Cuando systemd procesa
Entonces ejecuta /usr/bin/make -C /opt/services down
Y los 5 containers se detienen ordenadamente
Y volumes y network persisten
```

### Escenario 4: Type=oneshot + RemainAfterExit

```gherkin
Dado que `make up` termina exit 0 (containers en daemon mode)
Cuando systemd lo evalúa
Entonces Type=oneshot
Y RemainAfterExit=yes
Y el service queda en estado "active (exited)" sin reintentarse
```

### Escenario 5: EnvironmentFile carga .env

```gherkin
Dado que el unit corre como root
Cuando lee env vars
Entonces EnvironmentFile=/opt/services/.env
Y vars como DOMAIN_BACKEND_VERSION están disponibles
```

### Escenario 6: Timeouts razonables

```gherkin
Dado que el unit se evalúa
Cuando reviso timeouts
Entonces TimeoutStartSec=300 (5 min para arrancar todo)
Y TimeoutStopSec=120 (2 min para detener)
```

### Escenario 7: Logs visibles via journalctl

```gherkin
Dado que el unit arrancó
Cuando ejecuto `journalctl -u domain-services --since "5 min ago"`
Entonces veo output del make up (creating network, starting postgres, etc.)
Y errores eventuales del compose quedan capturados
```

## Notas

- El unit actual `domain-services.service` ya existe y llama a `docker compose
  up -d` directo. Esta HU lo redirige a `make up` para que la lógica de
  orquestación viva en un único lugar (Makefile, no duplicada en systemd).
- Los timers de backup y healthcheck (que ya existen) NO cambian.
