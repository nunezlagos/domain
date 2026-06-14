# Design: issue-38.10-installer-vps-refactor

## Decisión arquitectónica

- **Preservar todo el preflight existente** (Ubuntu + systemd + docker +
  auto-sudo + arch + compose validate). NO reescribir.
- **Sumar pasos**, no reemplazar.
- **Delegar orchestration al Makefile** (`make ensure-network`, `make up`).
- **Wait healthy de los 5 servicios** (no solo PG + MinIO).
- **Resumen final con URLs HTTP correctas** (sin TLS, sin dominio, solo IP).

## Alternativas descartadas

- **Reescribir install.sh completo**: ya está bien armado el flujo de
  preflight + auto-sudo + .env handling. Solo necesita extensiones puntuales.
- **Mover lógica de orchestration al install.sh**: duplica con Makefile.
  Mejor que install.sh delegue.
- **Pull paralelo de backend + frontend**: ahorro <30s, complejidad shell
  innecesaria. Secuencial OK.
- **Skip pull en re-installs**: requeriría detectar versión actual vs
  deseada. Mejor siempre pull (es idempotente y muestra "Image is up to
  date" si no hay update).

## Diff conceptual vs install.sh actual

```diff
  # ... preflight existente preservado (Ubuntu + systemd + docker + auto-sudo) ...
  
+ # Preflight extendido: validar composes nuevos
+ for compose_file in domain-backend/docker-compose.yml \
+                     domain-frontend/docker-compose.yml \
+                     caddy/docker-compose.yml; do
+   docker compose -f "$compose_file" --env-file .env.example config -q || {
+     fail "Compose inválido: $compose_file"; exit 1; }
+ done
+ ok "5 composes válidos"

  # ... pasos existentes (.env, certs, systemd) ...

+ step "X/Y  Pull imágenes Docker"
+ docker compose -f domain-backend/docker-compose.yml --env-file .env pull
+ docker compose -f domain-frontend/docker-compose.yml --env-file .env pull
+ ok "imágenes pulled"

  step "X/Y  Servicios"
- # ... antes: systemctl start; loop healthy (solo PG y MinIO) ...
+ make -C "$INSTALL_DIR" ensure-network
+ make -C "$INSTALL_DIR" up
+ # loop healthy de los 5
+ for i in {1..45}; do
+   sleep 2
+   healthy=$(docker ps --filter health=healthy --format '{{.Names}}' \
+            | grep -cE '^domain-(postgres|minio|backend|frontend|caddy)$' || true)
+   [[ "$healthy" -ge 5 ]] && { ok "5 healthy"; break; }
+   [[ $i -eq 45 ]] && warn "timeout"
+ done

  # ... resumen final cambiado ...
- # antes:
- #   Postgres: $VPS_IP:5432 ...
- #   MinIO: https://$VPS_IP:9000 ...
+ # después:
+ #   Dashboard: http://$VPS_IP/
+ #   API:       http://$VPS_IP/api/v1/...
+ #   MCP HTTP:  http://$VPS_IP/mcp
```

## Wait healthy: por qué 90s

5 servicios × ~15s healthy típico = 75s + buffer = 90s.
- PG: 10-15s (init + start_period)
- MinIO: 5s
- backend: 10-15s (boot + healthcheck)
- frontend: 5s (nginx ready)
- Caddy: 5-10s (config parse + reverse proxy check)

En VPS lento o con pull en curso, puede tomar más. Si timeout, `warn`
(no `fail`) y dejar al operador investigar con `make ps && make logs SVC=X`.

## Resumen final actualizado

```bash
cat <<RESUMEN

${GREEN}${BOLD}domain-services listo${RESET}

  Dashboard:  http://$VPS_IP/
  API:        http://$VPS_IP/api/v1/...
  MCP HTTP:   http://$VPS_IP/mcp
  Healthz:    http://$VPS_IP/healthz

  Backups:    diario 02:00 UTC → $INSTALL_DIR/backups/
  Alerts:     ntfy.sh/${NTFY_TOPIC:-<no-configurado>}

  Comandos útiles:
    cd $INSTALL_DIR
    make ps                    # estado de los 5
    make logs SVC=backend      # tail de uno
    make pull                  # tira imágenes nuevas
    make restart SVC=backend   # update sin tocar otros
    make backup                # backup manual
    make clean                 # DESTRUCTIVO

RESUMEN
```

## Idempotencia preservada

- Si `.env` existe: no se sobreescribe.
- Si network existe: `ensure-network` es no-op.
- Si imagen actualizada: pull la baja; up recrea container afectado.
- Si systemd ya instalado: cp lo reemplaza (mismas líneas).
- Re-correr completo: exit 0, sin daños.
