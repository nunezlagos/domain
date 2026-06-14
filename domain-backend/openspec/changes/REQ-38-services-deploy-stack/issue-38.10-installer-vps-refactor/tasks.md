# Tasks: issue-38.10-installer-vps-refactor

## Preflight (preservar + extender)

- [ ] **pre-001**: Confirmar que pasos existentes (root, OS Ubuntu, systemd,
      arch, docker, compose plugin) siguen presentes sin cambios.
- [ ] **pre-002**: Extender validación compose: agregar al loop existente
      los 3 composes nuevos (domain-backend, domain-frontend, caddy).
      Total validados: postgres, minio, domain-backend, domain-frontend, caddy.

## Pull imágenes (paso nuevo)

- [ ] **pull-001**: Sumar step "Pull imágenes Docker" antes del up:
      `docker compose -f domain-backend/... --env-file .env pull`
      `docker compose -f domain-frontend/... --env-file .env pull`
- [ ] **pull-002**: NO pull de postgres/minio/caddy (imágenes pinneadas
      estables; el .env los referencia).
- [ ] **pull-003**: Manejo de error: si pull falla, abortar con mensaje
      claro sugiriendo verificar .env y conectividad.

## Servicios up (extender)

- [ ] **up-001**: Reemplazar `systemctl start domain-services.service` por
      `make -C $INSTALL_DIR ensure-network` + `make -C $INSTALL_DIR up`.
- [ ] **up-002**: Loop wait-healthy extendido para chequear 5 containers
      (regex: `^domain-(postgres|minio|backend|frontend|caddy)$`).
- [ ] **up-003**: Timeout 90s (45 iter × 2s sleep).
- [ ] **up-004**: Si timeout: `warn` con sugerencia `make ps && make logs SVC=X`.

## Systemd (preservar)

- [ ] **sys-001**: `systemctl daemon-reload` sigue ejecutándose.
- [ ] **sys-002**: `systemctl enable` para los 3 units (service + 2 timers)
      sin cambios.
- [ ] **sys-003**: `systemctl start` de los 3 sin cambios.

## Resumen final (rewrite)

- [ ] **res-001**: Eliminar referencias a Postgres/MinIO con puertos públicos
      (ya no aplican).
- [ ] **res-002**: Sumar URLs: Dashboard, API, MCP HTTP, Healthz (todas
      http://$VPS_IP/...).
- [ ] **res-003**: Sumar comandos `make` actualizados: ps, logs SVC=X, pull,
      restart SVC=X, backup, clean.
- [ ] **res-004**: Backups: mantener mención (diario 02:00 UTC).
- [ ] **res-005**: Alerts: mantener mención (ntfy topic).

## Flags (preservar)

- [ ] **flag-001**: `--keep-clone` sigue funcionando.
- [ ] **flag-002**: `--skip-deps` sigue funcionando.
- [ ] **flag-003**: `--skip-compose-up` sigue funcionando (warn con `make up`
      como hint).

## Validación end-to-end

- [ ] **test-001**: VPS Ubuntu limpio: `./install.sh` corre OK en <5 min.
- [ ] **test-002**: Después de install: `make ps` muestra 5 containers healthy.
- [ ] **test-003**: `curl http://<vps-ip>/` devuelve placeholder HTML.
- [ ] **test-004**: `curl http://<vps-ip>/healthz` devuelve 200.
- [ ] **test-005**: `curl http://<vps-ip>/mcp` devuelve respuesta válida.
- [ ] **test-006**: `nc -zv <vps-ip> 80` exit 0; 5432 y 9000 fallan.
- [ ] **test-007**: `systemctl status domain-services` active.
- [ ] **test-008**: `systemctl status domain-services-backup.timer` active.
- [ ] **test-009**: Re-correr `./install.sh`: exit 0, idempotente.
- [ ] **test-010**: Editar .env (DOMAIN_BACKEND_VERSION=v1.2.4),
      `./install.sh`: pull actualiza backend, otros 4 intactos.
- [ ] **test-011**: `./install.sh --skip-compose-up`: configura systemd
      pero no levanta containers.
- [ ] **test-012**: `./install.sh --skip-deps`: asume docker presente.

## Notas para reviewers

- SOLO se edita `install.sh`. Cero otros archivos.
- TODO el preflight existente se preserva (no romper lo que funciona).
- Cambios son: extender compose validate, sumar pull, cambiar up a make,
  extender wait healthy, rewrite resumen final.
- Probar en VPS Ubuntu limpio (preferentemente Vagrant/multipass para
  reproducibilidad de tests).
