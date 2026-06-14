# domain-services

Infra para [domain](https://github.com/nunezlagos/domain): **Postgres (pgvector)** + **MinIO**, cada servicio en su propio `docker-compose.yml`.

## Instalación

```bash
git clone -b services <repo-url> /tmp/domain-services
cd /tmp/domain-services
./install.sh
```

Si no se corre como root, el script re-ejecuta con `sudo` automáticamente (1 sola contraseña). Verifica Ubuntu + systemd + docker, pide editar `/opt/services/.env` (passwords), genera certs TLS, instala units systemd y levanta los containers. Es idempotente.

Flags: `--keep-clone` · `--skip-deps` · `--skip-compose-up`.

## Layout

```
postgres/   docker-compose.yml + config + init scripts
minio/      docker-compose.yml
scripts/    gen-certs.sh · backup.sh · healthcheck-alert.sh
systemd/    units (boot · backup diario · healthcheck cada 5min)
.env        passwords (chmod 600, nunca committear)
```

## Operación

```bash
cd /opt/services
make up                 # ambos
make up SVC=postgres    # solo PG
make up SVC=minio       # solo MinIO
make ps
make logs SVC=postgres
make backup
make psql
make certs              # renueva si vence en <30 días
make clean              # DESTRUCTIVO
```

## Acceso desde la laptop

- **Postgres**: `<vps-ip>:5432`, `sslmode=require`. Roles: `app_user`, `app_admin`, `app_migrator`.
- **MinIO**: `https://<vps-ip>:9000` (API), `http://<vps-ip>:9001` (UI). Bucket: `domain-attachments`.

## Backups

Diario 02:00 UTC → `/opt/services/backups/` (pg_dump GPG-AES256 + mirror MinIO). Retención en `.env`.

Restaurar Postgres:
```bash
gpg -d /opt/services/backups/postgres/YYYY-MM-DD.sql.gz.gpg | gunzip \
  | docker exec -i domain-postgres psql -U postgres -d domain
```

## Update

```bash
cd /opt/services && git pull && ./install.sh
```
