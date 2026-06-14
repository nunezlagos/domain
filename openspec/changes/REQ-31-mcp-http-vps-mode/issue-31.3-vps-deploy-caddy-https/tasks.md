# Tasks: issue-31.3-vps-deploy-caddy-https

## Backend

- [ ] **T1**: Crear estructura `deploy/contabo/` con:
  - `setup.sh` (entrypoint).
  - `lib/common.sh`, `lib/checks.sh`, `lib/secrets.sh`,
    `lib/caddy.sh`, `lib/compose.sh` (helpers bash).
  - `caddy/Caddyfile.template`.
  - `compose/docker-compose.yml` (production-ready).
  - `backup/backup.sh`, `backup/backup.cron`.
  - `README.md` con how-to completo.

- [ ] **T2**: `lib/checks.sh`:
  - `check_root` — error si no corre como root.
  - `check_os` — Ubuntu 24.04 (o compatible). Warning en otros.
  - `check_docker` — `docker --version` debe existir; si no,
    instalar con el script oficial get-docker.sh.
  - `check_dns(domain)` — `dig +short $domain` debe retornar la
    IP del VPS. Comparar con `hostname -I`. Si no match, abortar
    con mensaje claro.

- [ ] **T3**: `lib/secrets.sh`:
  - `ensure_secrets(envfile)` — si el archivo no existe, generar
    con `openssl rand -base64 32` para cada key requerida
    (DOMAIN_MASTER_KEY, POSTGRES_PASSWORD, MINIO_ROOT_PASSWORD,
    JWT_SECRET). chmod 600. Si existe, no tocar.

- [ ] **T4**: `lib/caddy.sh`:
  - `write_caddyfile(output, domain)` — lee el template, sustituye
    `{$DOMAIN:...}` con el dominio real, escribe con header
    `Strict-Transport-Security` y otros.

- [ ] **T5**: `compose/docker-compose.yml`:
  - Services: caddy, domain, postgres, minio.
  - Postgres y minio SIN `ports:` (solo network interno).
  - Domain SIN `ports:` (Caddy lo proxyifica).
  - Caddy con `ports: ["443:443", "80:80"]`.
  - Volúmenes nombrados para data persistencia.
  - `restart: unless-stopped` en todos.

- [ ] **T6**: `lib/compose.sh`:
  - `up(dir)` — `docker compose -f $dir/compose/docker-compose.yml
    --env-file $dir/.env up -d`.
  - `down(dir)` — para rollback.
  - `healthcheck(dir, timeout)` — espera `up` + curl `/health` en
    loop con timeout 30s.

- [ ] **T7**: `backup/backup.sh`:
  - `pg_dump | gzip > /backups/db-$(date +%Y%m%d).sql.gz`.
  - Retiene últimos 7 (borra los más viejos).
  - Exit 0 si OK, !=0 si falla.

- [ ] **T8**: `backup/backup.cron`:
  - Línea: `0 3 * * * root /opt/domain/deploy/contabo/backup/backup.sh
    >> /var/log/domain-backup.log 2>&1`.
  - `setup.sh` lo instala con `crontab -u root backup.cron`.

- [ ] **T9**: `setup.sh` flow:
  1. Parse arg `<domain>` (required).
  2. `check_root`, `check_os`, `check_docker`.
  3. `check_dns $domain`.
  4. Crear `/opt/domain/`, `git clone` si no existe.
  5. `ensure_secrets /opt/domain/.env`.
  6. `write_caddyfile /opt/domain/caddy/Caddyfile $domain`.
  7. `up /opt/domain`.
  8. `healthcheck /opt/domain 30s`.
  9. `install_cron`.
  10. Print summary con next steps.

- [ ] **T10**: `README.md` con:
  - Prereqs (VPS Contabo, dominio, DNS configurado).
  - Walkthrough de `bash deploy/contabo/setup.sh api.tudominio.com`.
  - Cómo verificar (curl health, nmap, ls /backups).
  - Cómo rollback (`bash deploy/contabo/setup.sh --rollback`).
  - Cómo rotar secrets (procedimiento manual + script).
  - Troubleshooting (DNS no propagado, ACME falla, etc).

## Tests

- [ ] **T-unit-1**: `TestCheckDNS_Match**` — mock `dig` que retorna
  la IP del VPS → check pasa.
- [ ] **T-unit-2**: `TestCheckDNS_Mismatch**` — mock que retorna
  otra IP → check aborta con mensaje claro.
- [ ] **T-unit-3**: `TestEnsureSecrets_GeneratesIfMissing**` —
  envfile no existe → se crea con 4 keys, chmod 600.
- [ ] **T-unit-4**: `TestEnsureSecrets_PreservesIfExists**` —
  envfile existe → no se regenera, contenido idéntico.
- [ ] **T-unit-5**: `TestWriteCaddyfile_SubsDomain**` — template con
  placeholder `{$DOMAIN:...}` + domain arg → output tiene el
  dominio real.
- [ ] **T-e2e-1**: `TestSetup_LocalDockerCompose**` — setup.sh con
  `--dry-run` (no toca Docker real, solo escribe archivos) →
  Caddyfile + .env + cron instalados correctamente.
- [ ] **T-e2e-2**: `TestCompose_NoPublicPortsForPostgres**` — el
  compose final no tiene `ports:` para postgres ni minio (grep
  verifica).
- [ ] **T-e2e-3**: `TestBackup_RetainsLast7**` — backup.sh con 10
  backups previos → deja solo 7 más recientes.
- [ ] **T-sabotaje**: Cambiar Caddyfile template para apuntar a
  `localhost:9999` (puerto equivocado) → `write_caddyfile` →
  test e2e-1 DEBE detectar que el upstream port no es 8000 →
  restaurar puerto correcto → test verde. Documentar en commit
  body.
