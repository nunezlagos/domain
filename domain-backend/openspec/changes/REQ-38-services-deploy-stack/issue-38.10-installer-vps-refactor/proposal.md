# Proposal: issue-38.10-installer-vps-refactor

## Intención

Extender el `install.sh` actual para que pull imágenes desde GHCR, asegure
la red `domain_internal`, levante los 5 servicios via `make`, instale los
units systemd actualizados, y muestre un resumen final con las URLs HTTP
correctas del VPS.

## Scope

**Incluye:**
- Mantener TODO el preflight existente (Ubuntu + systemd + docker + auto-sudo +
  arch + compose validate).
- Sumar al preflight: validar los 3 composes nuevos
  (`domain-backend/docker-compose.yml`, `domain-frontend/docker-compose.yml`,
  `caddy/docker-compose.yml`) con `docker compose config -q`.
- Sumar paso "Pull imágenes":
  ```
  step "X/Y  Pull imágenes GHCR"
  docker compose -f domain-backend/docker-compose.yml --env-file .env pull
  docker compose -f domain-frontend/docker-compose.yml --env-file .env pull
  ```
- Sumar paso "Ensure network" antes del up: `make ensure-network`.
- Cambiar el paso de levantar servicios para usar `make up` (no `docker
  compose up` directo).
- Cambiar el wait-healthy para chequear los 5 containers (no solo PG y MinIO).
- Cambiar el resumen final:
  - Mostrar `http://<vps-ip>/`, `http://<vps-ip>/api/v1/...`, `http://<vps-ip>/mcp`
  - NO mencionar puertos públicos de PG/MinIO (ya no aplican)
  - Mostrar comandos make actualizados (ps, logs SVC, backup, restart SVC, pull)

**No incluye:**
- Generación de secrets (sigue siendo manual via `openssl rand`).
- Setup de TLS/dominio (decisión cerrada: HTTP plano por IP).
- Configuración del cliente MCP (eso es REQ-CLIENT futuro, fuera de scope).

## Enfoque técnico

1. **Preservar lo bueno**: no tocar la lógica de preflight (Ubuntu check,
   systemd check, arch, auto-sudo, .env handling). Sumar pasos, no
   reescribir.

2. **Nuevo paso de pull (antes del up)**:
   ```bash
   step "X/Y  Pull imágenes Docker"
   docker compose -f domain-backend/docker-compose.yml --env-file .env pull
   docker compose -f domain-frontend/docker-compose.yml --env-file .env pull
   ok "imágenes actualizadas"
   ```

3. **Cambio del paso "Servicios"**:
   ```bash
   step "X/Y  Servicios"
   if [[ $SKIP_COMPOSE_UP -eq 1 ]]; then
     warn "skip (corré: make up)"
   else
     make ensure-network
     make up
     ok "5 servicios up"

     echo "    Esperando healthy..."
     for i in {1..45}; do
       sleep 2
       healthy=$(docker ps --filter health=healthy --format '{{.Names}}' \
                | grep -cE '^domain-(postgres|minio|backend|frontend|caddy)$' || true)
       [[ "$healthy" -ge 5 ]] && { ok "los 5 healthy"; break; }
       [[ $i -eq 45 ]] && warn "timeout esperando healthy; revisar make ps + make logs SVC=<svc>"
     done
   fi
   ```

4. **Resumen final ajustado**:
   ```bash
   cat <<RESUMEN

   ${GREEN}${BOLD}domain-services listo${RESET}

     Dashboard:  http://$VPS_IP/
     API:        http://$VPS_IP/api/v1/...
     MCP HTTP:   http://$VPS_IP/mcp
     Healthz:    http://$VPS_IP/healthz

     Backups:    diario 02:00 UTC → $INSTALL_DIR/backups/
     Alerts:     ntfy.sh/${NTFY_TOPIC:-<no-configurado>}

     cd $INSTALL_DIR && make {ps,logs SVC=X,backup,pull,restart SVC=X}

   RESUMEN
   ```

5. **Idempotencia preservada**: re-correr install.sh no rompe nada. Si
   `.env` existe, no se sobrescribe. Si network existe, no se recrea.
   Si imágenes actualizadas, `pull` las baja; `make up` recrea solo
   containers afectados.

## Riesgos

- **Pull falla por imagen inexistente en GHCR**: si CI todavía no publicó
  versión X, `docker compose pull` falla. Mitigación: el preflight valida
  composes pero no pull; pull explícito está en su propio step. Si falla,
  el script aborta y el operador ajusta `.env` con versión correcta.
- **Wait healthy timeout**: 90s puede ser insuficiente en VPS lento o con
  red mala (pull tarda). Mitigación: subir a 180s o desacoplar pull del up.
- **Operador re-corre install y borra worktree**: el flag `--keep-clone`
  protege; sin él, install borra el directorio fuente (que puede ser el
  /tmp/domain-services del clone). Mitigación: documentar bien.

## Testing

- VPS Ubuntu limpio: `./install.sh` corre end-to-end en <5 min
- Después del install: `make ps` muestra 5 containers healthy
- `curl http://<vps-ip>/` devuelve HTML del placeholder
- `curl http://<vps-ip>/healthz` devuelve 200
- `curl http://<vps-ip>/mcp` devuelve respuesta válida del MCP handler
- `nc -zv <vps-ip> 5432` falla (PG no expuesto)
- `nc -zv <vps-ip> 9000` falla (MinIO no expuesto)
- `nc -zv <vps-ip> 80` exit 0 (único puerto público)
- `systemctl status domain-services` muestra active
- `systemctl status domain-services-backup.timer` muestra active
- Re-correr `./install.sh` → exit 0, idempotente
- Editar `.env` con `DOMAIN_BACKEND_VERSION=v1.2.4`, re-correr install →
  pull + recrea container backend, otros 4 intactos
- `--skip-compose-up` configura todo sin levantar
- `--skip-deps` asume docker ya presente, no apt install
