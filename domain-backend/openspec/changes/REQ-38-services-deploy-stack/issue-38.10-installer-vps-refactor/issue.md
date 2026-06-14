# issue-38.10-installer-vps-refactor

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** alta
**Tipo:** infrastructure / installer
**Wave:** 4 (depende de 38.6, 38.7, 38.8, 38.9)

## Historia de usuario

**Como** operador que recibe un VPS Ubuntu limpio
**Quiero** correr `./install.sh` y que el script verifique pre-requisitos,
levante los 5 servicios, configure systemd timers de backup y healthcheck,
y deje todo funcionando con el dashboard accesible en `http://<vps-ip>/`
**Para** no tener que conocer la orquestación interna ni los comandos
docker/make/systemctl manualmente.

## Criterios de aceptación

### Escenario 1: Preflight extendido (mantiene el de antes + verifica nuevos servicios)

```gherkin
Dado que corro `./install.sh` en Ubuntu limpio
Cuando ejecuta paso de preflight
Entonces verifica OS = Ubuntu, systemd presente, arch amd64|arm64 (ya existe)
Y verifica que `docker compose config -q` pasa para los 5 composes:
  postgres, minio, domain-backend, domain-frontend, caddy
Y falla rápido si alguno tiene YAML inválido
```

### Escenario 2: Pull de imágenes antes del up

```gherkin
Dado que .env tiene DOMAIN_BACKEND_VERSION=v1.2.3
Cuando install.sh hace pull
Entonces docker compose -f domain-backend/docker-compose.yml pull
Y docker compose -f domain-frontend/docker-compose.yml pull
Y postgres, minio, caddy NO se pullean (imágenes pinneadas estables)
Y el output muestra el progreso del pull con tamaño
```

### Escenario 3: ensure-network antes del up

```gherkin
Dado que `domain_internal` no existe
Cuando install.sh ejecuta el up
Entonces primero crea la red con docker network create domain_internal
Y luego procede con make up
```

### Escenario 4: Auto-sudo preservado

```gherkin
Dado que el user corre `./install.sh` sin sudo
Cuando install.sh detecta EUID != 0
Entonces ejecuta `exec sudo -E bash $0 $@`
Y pide contraseña UNA sola vez
Y el resto corre como root sin más prompts
```

### Escenario 5: systemd units instalados y enabled

```gherkin
Dado que paso del up exitoso
Cuando install.sh procesa systemd/
Entonces copia las 5 units a /etc/systemd/system/
Y systemctl daemon-reload
Y systemctl enable domain-services.service
Y systemctl enable domain-services-backup.timer
Y systemctl enable domain-services-healthcheck.timer
Y systemctl start de todos
```

### Escenario 6: Wait healthy de los 5 con timeout

```gherkin
Dado que los containers arrancan
Cuando install.sh espera healthy
Entonces espera hasta 90s a que los 5 reporten healthy
Y los 5 son: domain-postgres, domain-minio, domain-backend, domain-frontend, domain-caddy
Y si alguno no llega: imprime warning con `make logs SVC=<svc>` como hint
Y NO aborta (sigue al resumen)
```

### Escenario 7: Resumen final con URLs correctas

```gherkin
Dado que todo está arriba
Cuando install.sh muestra el resumen final
Entonces imprime:
  - Dashboard: http://<vps-ip>/
  - API:       http://<vps-ip>/api/v1/...
  - MCP:       http://<vps-ip>/mcp
  - Health:    http://<vps-ip>/healthz
Y NO menciona PG ni MinIO con puertos públicos (ya no son accesibles externos)
Y muestra comandos make útiles: ps, logs, backup, restart, clean
```

### Escenario 8: Re-correr es idempotente

```gherkin
Dado que install.sh ya corrió y todo está arriba
Cuando vuelvo a ejecutar ./install.sh
Entonces NO sobrescribe .env (si existe)
Y NO recrea network ni volumes
Y hace pull de imágenes nuevas si DOMAIN_*_VERSION cambió
Y restart de los services que tienen imagen nueva
Y termina con exit 0
```

### Escenario 9: Flags preservadas

```gherkin
Dado que el script soporta flags
Cuando paso --skip-deps, --skip-compose-up, --keep-clone
Entonces --skip-deps no instala docker (asume presente)
Y --skip-compose-up configura pero no levanta containers
Y --keep-clone no borra el directorio fuente al final
```

## Notas

- install.sh actual ya tiene preflight + auto-sudo + systemd + .env handling.
  Esta HU lo EXTIENDE para los 5 servicios + pull GHCR + ensure-network.
- El script debe seguir corriendo en <5 min en VPS típico con red razonable.
- No hay TLS, no hay dominio: el resumen final habla de `http://<ip>/` plano.
