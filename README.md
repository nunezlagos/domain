# domain-services

Infraestructura cloud para [domain](https://github.com/nunezlagos/domain): **Postgres + MinIO**
ready-to-deploy en un VPS Ubuntu.

Esta rama (`services`, orphan del repo principal) **no contiene código Go** —
solo lo necesario para parar la infra del VPS. Las migrations y seeders los
corre el instalador de domain desde la laptop del dev con el rol `app_migrator`.

```
┌─ laptop dev ────────────┐         ┌─ VPS Ubuntu ──────────────────┐
│  domain (binary)        │   TCP   │  Postgres :5432 (TLS)         │
│  domain-mcp             │ ──────► │   • pgvector pg16             │
│  opencode / claude-code │         │   • roles app_user/admin/...  │
│                         │   HTTP  │                               │
│                         │ ──────► │  MinIO :9000 (TLS)            │
│                         │         │   • bucket domain-attachments │
│                         │         │   • UI :9001 (http)           │
└─────────────────────────┘         └───────────────────────────────┘
```

## Servicios incluidos

| Servicio | Imagen | Puerto host | TLS |
|---|---|---|---|
| **Postgres** | `pgvector/pgvector:pg16` | 5432 | self-signed |
| **MinIO** | `minio/minio:RELEASE.2024-10-13` | 9000 (API) + 9001 (UI) | self-signed (9000); UI plain |

Meilisearch y Caddy quedaron afuera del scope inicial. Ver issue tracker para roadmap.

## Instalación en el VPS

### Bootstrap rápido

```bash
# 1) Clonar la rama services (en cualquier carpeta del VPS)
git clone -b services <repo-url> /tmp/domain-services

# 2) Bootstrap (root)
cd /tmp/domain-services
sudo ./install.sh
```

El installer hace todos los pasos:

1. Verifica root
2. Instala docker + docker compose plugin + openssl + gpg
3. Copia los archivos a `/opt/services/` (donde docker compose los leerá)
4. Te pide editar `/opt/services/.env` (passwords, IPs, ntfy topic)
5. Genera certs TLS self-signed (CN = IP pública del VPS)
6. Instala unidades systemd (auto-start, backup diario, healthcheck cada 5min)
7. Levanta los containers
8. Elimina el clone temporal (`--keep-clone` para mantenerlo)

Flags útiles:
- `--keep-clone` no borra `/tmp/domain-services` al final
- `--skip-deps` si ya tenés docker instalado
- `--skip-compose-up` configura todo pero no levanta containers

### Re-correr install.sh

Es **idempotente**: re-correrlo no borra data ni sobreescribe el `.env`. Útil para
aplicar cambios después de editar archivos en `/opt/services/`.

## Operación día-a-día

Todo desde `/opt/services` con `make`:

```bash
make up               # levanta todos los servicios (profile core)
make up SVC=postgres  # solo postgres
make down             # detiene todo (mantiene volumes)
make restart          # reinicia
make ps               # estado containers
make logs             # tail de todos
make logs SVC=minio   # tail de uno
make psql             # shell SQL adentro del postgres container
make mc               # cliente MinIO interactivo
make backup           # backup manual (normalmente corre automático a las 02:00)
make certs            # regenera certs si están por expirar
make certs-force      # fuerza regeneración
make clean            # DESTRUCTIVO: borra todos los volumes (requiere confirmar)
```

## Roles en Postgres

Creados automáticamente por `postgres/init/02-roles.sh` en el primer boot:

| Rol | RLS | Privilegios | Uso |
|---|---|---|---|
| `app_user` | NOBYPASSRLS | CRUD en tables del schema public | runtime queries de domain |
| `app_admin` | BYPASSRLS | ALL en tables | auth/audit lookups (org_id aún no conocido) |
| `app_migrator` | NOBYPASSRLS | CREATE en schema public | golang-migrate desde la laptop |
| `postgres` (superuser) | bypass | ALL | admin manual, NO usar desde la app |

Ver `.claude/rules/connection-pools.md` en el repo principal de domain.

Conectarse desde domain (laptop):

```bash
# Migrations (rol app_migrator)
DOMAIN_DATABASE_URL="postgres://app_migrator:***@<vps-ip>:5432/domain?sslmode=require" \
  domain migrate up

# Runtime queries (rol app_user)
DOMAIN_DATABASE_URL="postgres://app_user:***@<vps-ip>:5432/domain?sslmode=require"

# Auth/audit (rol app_admin, solo donde org_id es desconocido)
DOMAIN_DATABASE_AUTH_URL="postgres://app_admin:***@<vps-ip>:5432/domain?sslmode=require"
```

**Importante:** las passwords de los roles vienen del `.env` del VPS. Coordiná con
quien admin el VPS para obtenerlas (o las generás vos al deployar).

## Seguridad

Riesgos asumidos en este setup:

- **Postgres expuesto en internet** con TLS self-signed + SCRAM-SHA-256.
  Defensa: passwords fuertes (64 chars random). Sin firewall por IP (rechazado por
  el operador). Hay que aceptar manualmente que la BD aparece en escaneos públicos.
- **MinIO expuesto en internet** con TLS self-signed + access keys.
- **No hay dominio** → certs auto-firmados; los clientes deben pasar `-k` (curl) o
  `sslmode=require` sin cert verification (`sslrootcert=` opcional para pinning).

Si en algún momento el operador acepta usar Tailscale, los servicios pasarían
a bind solo a `tailscale0` y el riesgo desaparece. Ver issue tracker.

### Generar passwords fuertes

```bash
openssl rand -base64 48 | tr -d '/+=' | head -c 32
```

Una password por rol (postgres, app_user, app_admin) + una para minio admin + una
para `BACKUP_GPG_PASSPHRASE`.

## Backups

- **Schedule:** diario 02:00 UTC vía `domain-services-backup.timer` (systemd)
- **Contenido:** `pg_dump` cifrado con GPG (AES-256, passphrase del `.env`) + mirror
  de buckets MinIO
- **Destino:** `/opt/services/backups/`
- **Rotación:** 7 daily (config en `.env`)
- **Notificación:** post-run manda a ntfy.sh si está configurado

### Restaurar

```bash
# Postgres (en el host)
gpg --decrypt /opt/services/backups/postgres/2026-06-13.sql.gz.gpg \
  | gunzip \
  | docker exec -i domain-postgres psql -U postgres -d domain

# MinIO
docker exec -i domain-minio mc mirror /backups/minio/2026-06-13/ local/<bucket>/
```

## Healthcheck + alerting

Cada 5min un script chequea el estado de cada container y notifica via [ntfy.sh](https://ntfy.sh)
si algo falla.

Configurar en `.env`:
```
NTFY_TOPIC=domain-vps-alerts-7f3a9c
```

Suscribirse desde la app ntfy (Android/iOS/web) al mismo topic. Recibís push si
algún servicio se cae o queda unhealthy, y otro push cuando se recupera.

## Update flow

Para aplicar cambios a la infra:

```bash
# 1) En tu laptop (o cualquier máquina con el repo)
git checkout services
# editar archivos
git commit -am "...."
git push

# 2) En el VPS
cd /opt/services
git pull origin services      # (requiere que /opt/services tenga remote configurado)
sudo ./install.sh             # idempotente, aplica cambios
```

Alternativa más simple si la branch no tiene remote en el VPS: re-clonar y re-bootstrap:

```bash
sudo rm -rf /tmp/domain-services
git clone -b services <repo> /tmp/domain-services
cd /tmp/domain-services
sudo ./install.sh
```

`install.sh` detecta `/opt/services` existente y hace merge sin tocar volumes ni `.env`.

## Estructura del repo

```
.
├── docker-compose.yml          # PG + MinIO + minio-bootstrap (one-shot)
├── .env.example                # template de secrets
├── Makefile                    # comandos día-a-día
├── install.sh                  # bootstrap del VPS
├── README.md                   # este archivo
├── postgres/
│   ├── postgresql.conf         # SSL on, listen_addresses=*, logging
│   ├── pg_hba.conf             # hostssl scram-sha-256 only
│   └── init/
│       ├── 01-extensions.sql   # pgvector, pgcrypto, pg_trgm, etc.
│       └── 02-roles.sh         # app_user / app_admin / app_migrator
├── minio/                      # placeholder (config via env vars)
├── scripts/
│   ├── gen-certs.sh            # certs TLS self-signed
│   ├── backup.sh               # pg_dump + mc mirror + rotación
│   └── healthcheck-alert.sh    # ntfy notification si algo down
└── systemd/
    ├── domain-services.service              # boot up
    ├── domain-services-backup.service       # one-shot backup
    ├── domain-services-backup.timer         # daily 02:00
    ├── domain-services-healthcheck.service  # one-shot check
    └── domain-services-healthcheck.timer    # cada 5min
```

## Decisiones de diseño (FAQ)

**¿Por qué orphan branch en vez de repo separado?**
Un solo origen de verdad. Sync de versiones entre code y deploy queda implícito por
el repo común.

**¿Por qué no Tailscale?**
El operador prefiere acceso por IP pública para no depender de un cliente VPN cuando
viaja. Se asume el riesgo de exposición pública + se compensa con TLS + passwords
fuertes.

**¿Por qué no Caddy / TLS Let's Encrypt?**
No hay dominio registrado. Self-signed cubre tráfico en tránsito; el resto es
auth y passwords.

**¿Por qué systemd en vez de cron?**
Mejor logging (`journalctl -u domain-services-backup`), re-run si el VPS estaba
apagado (con `Persistent=true`), y status visible con `systemctl status`.

**¿Por qué no incluir el binario domain?**
Decisión actual: domain corre en la laptop del dev (donde opencode/claude-code
edita agents). Cuando REQ-31 (MCP HTTP mode) esté implementado, vamos a poder
correr domain también en el VPS y los servicios pasarán a privados.

## Troubleshooting

### "no pude detectar la IP pública"

```bash
# Detectarla manualmente
curl ifconfig.me
# y ponerla en .env
VPS_PUBLIC_IP=1.2.3.4
```

### Postgres no arranca: "permission denied" en server.key

Los certs montados como bind-mount necesitan que el usuario `postgres` (uid 999 en
el container pgvector) los pueda leer:

```bash
sudo chown -R 999:999 /opt/services/certs/postgres
sudo chmod 600 /opt/services/certs/postgres/server.key
```

### Conexión externa rechazada

Verificar:
1. El puerto está en el `ports:` del compose (no comentado)
2. El firewall del VPS (ufw, security group de cloud) permite TCP/5432 y TCP/9000
3. Postgres está healthy: `docker ps` y `docker logs domain-postgres`
4. Cliente usa `sslmode=require` (no `disable`)

### Cert expiró

```bash
cd /opt/services
sudo make certs        # regenera si <30 días para expirar
sudo make certs-force  # regenera siempre
sudo make restart      # reinicia containers para que tomen los nuevos
```

## Soporte

Issues: en el repo principal `domain`, tag `infra` o `req-31`.
