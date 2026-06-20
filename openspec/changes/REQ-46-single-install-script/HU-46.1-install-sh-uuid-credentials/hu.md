# HU-46.1-install-sh-uuid-credentials

**Origen:** `REQ-46-single-install-script`
**Prioridad tentativa:** alta
**Tipo:** refactor + feature

## Historia de usuario
**Como** operador que clona el repo en un VPS Ubuntu
**Quiero** UN solo `services/install.sh` que se ocupe de TODO (validar OS, instalar docker, generar credenciales UUID, deployar, configurar systemd timers, imprimir credenciales)
**Para** tener un único punto de entrada, sin sshpass/rsync local, sin archivos de credenciales en el dev, y que el reinstall preserve las credenciales existentes

## Criterios de aceptación

- Existe un único `services/install.sh` que es el bootstrap del stack
- Credenciales (POSTGRES_PASSWORD, APP_USER_PASSWORD, APP_ADMIN_PASSWORD, MINIO_ROOT_PASSWORD, BACKUP_GPG_PASSPHRASE) se generan como UUID v4 en install fresco
- Si `.env` ya existe en `/opt/services/.env`, las credenciales se PRESERVAN (no se regeneran)
- Al finalizar, se imprimen TODAS las credenciales por consola con formato legible
- El install funciona tanto en VPS vacío como en VPS con servicios ya corriendo (idempotente)
- `install.sh` reemplaza a: `install-vps.sh`, `scripts/deploy-vps.sh`, `.env.vps`
- Se pueden borrar los archivos legacy (HU-46.2)

## Análisis breve

- **Qué pide realmente:** consolidar el bootstrap en un script único, mover la generación de credenciales al install-time, y hacer el reinstall idempotente preservando secrets.
- **Módulos sospechados:** `services/install.sh` (nuevo), borrar `services/install-vps.sh`, borrar `scripts/deploy-vps.sh`, borrar `.env.vps`
- **Riesgos / dependencias:**
  - Si el install falla a mitad (ej: docker compose falla), las credenciales ya están generadas. Re-correr install las preserva. OK.
  - Si rotás credenciales manualmente en `.env` y re-corrés install, se preservan tus valores. OK.
  - UUIDs se generan con `/dev/urandom` (sin dependencia de `uuidgen` o python)
- **Esfuerzo tentativo:** M

## Verificación previa

- [x] Revisar codebase (grep) — `install-vps.sh` y `deploy-vps.sh` ya existen, son redundantes
- [x] Revisar Caddyfile/Makefile — usan `services/` subpath, install.sh debe respetar eso
- [ ] Probar en ambiente correcto (corriéndolo en el VPS real)
- [ ] Verificar preservado de credenciales en reinstall

### Resultado de verificación

- **Estado:** listo para implementar
- **Acción derivada:** crear install.sh único con UUID gen + preservado